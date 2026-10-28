package tracker

import (
	"io"
	"math"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	mp3 "github.com/dece2183/go-stream-mp3"
	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/equalizer"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/stream"
)

const (
	_PROGRESS_UPDATE_PERIOD = 250 * time.Millisecond

	// _LOUDNESS_ANALYSIS_SECONDS is how much of a track is decoded to measure
	// its loudness on the very first play. The head of a track is only an
	// approximation (music usually gets louder after the intro), so the exact
	// loudness is measured while the track plays and remembered for the next
	// plays: such tracks start with no delay at all.
	_LOUDNESS_ANALYSIS_SECONDS = 10
)

type readWrapper struct {
	program        *tea.Program
	decoder        *mp3.Decoder
	trackBuffer    *stream.BufferedStream
	trackBuffered  bool
	trackDone      bool
	lastUpdateTime time.Time
	normalization  normalization
	measurePending bool
	duckLogged     bool
	trackID        string
	trackTitle     string
	// playMeter measures the whole track while it plays: the result is exact
	// and is remembered for the next plays.
	playMeter *loudnessMeter
	// gainDB keeps the applied correction as the bits of a float64, so the UI
	// can read it without touching the audio thread.
	gainDB uint64
	// eq applies the equalizer to the decoded PCM.
	eq *equalizer.Processor
}

func (w *readWrapper) NewReader(reader *stream.BufferedStream, track *api.Track) error {
	var err error

	w.trackBuffered = false
	w.trackDone = false
	w.trackBuffer = reader
	w.playMeter = nil
	w.duckLogged = false
	w.trackID = ""
	if track != nil {
		w.trackID = track.Id
		w.trackTitle = track.Title
	}

	w.normalization, w.measurePending = newNormalization(track)

	if w.eq == nil {
		w.eq = equalizer.New()
	} else {
		w.eq.Reset()
	}

	// a track that was played to the end before has its loudness measured
	// exactly: it needs neither the analysis nor the delay it costs
	if w.measurePending {
		if entry, ok := lookupLoudness(w.trackID); ok {
			w.measurePending = false
			w.normalization = normalizationFor(entry.Loudness, entry.Peak)
			log.Print(log.LVL_INFO, "normalization: track '%s' %.1f LUFS (measured before), gain %+.1f dB",
				w.trackTitle, entry.Loudness, w.normalization.gainDB())
		}
	}

	atomic.StoreUint64(&w.gainDB, math.Float64bits(w.normalization.gainDB()))

	w.decoder, err = mp3.NewDecoder(w.trackBuffer)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to create mp3 decoder: %s", err)
		w.decoder = nil
		return err
	}

	w.lastUpdateTime = time.Now()

	return nil
}

// measureLoudness decodes the beginning of the track to find out how loud it
// is, rewinds the stream and hands the correction to the audio callback. It
// runs on the player thread, before the first samples are played.
func (w *readWrapper) measureLoudness() {
	if w.decoder == nil || w.trackBuffer == nil {
		return
	}

	meter := newLoudnessMeter(w.decoder.SampleRate())
	bytesPerSecond := meter.sampleRate * 4
	buffer := make([]byte, 32*1024)
	wanted := bytesPerSecond * _LOUDNESS_ANALYSIS_SECONDS
	read := 0

	for read < wanted {
		n, err := w.decoder.Read(buffer)
		if n > 0 {
			meter.add(buffer[:n])
			read += n
		}
		if err != nil {
			break
		}
	}

	// the analysis has consumed the head of the stream: put it back and start
	// decoding from the very first frame again
	if _, err := w.trackBuffer.Seek(0, io.SeekStart); err != nil {
		log.Print(log.LVL_WARNIGN, "normalization: unable to rewind the track: %s", err)
		return
	}

	decoder, err := mp3.NewDecoder(w.trackBuffer)
	if err != nil {
		log.Print(log.LVL_WARNIGN, "normalization: unable to restart the decoder: %s", err)
		return
	}
	w.decoder = decoder

	w.normalization = meter.normalization()
	atomic.StoreUint64(&w.gainDB, math.Float64bits(w.normalization.gainDB()))

	seconds := read / bytesPerSecond
	if loudness := meter.integratedLoudness(); math.IsInf(loudness, -1) {
		log.Print(log.LVL_INFO, "normalization: track '%s' is silent, played as is", w.trackTitle)
	} else {
		log.Print(log.LVL_INFO, "normalization: track '%s' %.1f LUFS in the first %d s (peak %.2f), gain %+.1f dB",
			w.trackTitle, loudness, seconds, meter.peak, w.normalization.gainDB())
	}
}

// finalizeLoudness remembers the loudness of a track that was played to the
// end: the next play of it will start with the exact correction and without
// the analysis.
func (w *readWrapper) finalizeLoudness() {
	if w.playMeter == nil || len(w.playMeter.windows) < _LOUDNESS_MIN_WINDOWS {
		return
	}

	loudness := w.playMeter.integratedLoudness()
	if math.IsInf(loudness, 0) {
		return
	}

	log.Print(log.LVL_INFO, "normalization: track '%s' measured %.1f LUFS in full (peak %.2f)",
		w.trackTitle, loudness, w.playMeter.peak)
	storeLoudness(w.trackID, loudnessEntry{Loudness: loudness, Peak: w.playMeter.peak})
}

func (w *readWrapper) Close() {
	if w.decoder != nil {
		w.decoder.Seek(0, io.SeekStart)
	}

	if w.trackBuffer != nil {
		w.trackBuffer.Close()
	}
}
func (w *readWrapper) Read(dest []byte) (n int, err error) {
	if w.trackBuffer == nil {
		err = io.EOF
		return
	}

	// the decoder is missing when the stream was empty or broken: the player
	// must get the end of the track instead of a crash
	if w.decoder == nil {
		err = io.EOF
		return
	}

	// the loudness of the track is measured once, before the first samples
	// reach the player
	if w.measurePending {
		w.measurePending = false
		w.measureLoudness()
	}

	n, err = w.decoder.Read(dest)
	if err != nil && err != io.EOF {
		if w.trackBuffer.Error() != nil {
			err = w.trackBuffer.Error()
			log.Print(log.LVL_ERROR, "buffering error: %s", err)
			go w.program.Send(STOP)
			return
		}
		// bypass mp3 decoding error after rewinding
		log.Print(log.LVL_WARNIGN, "mp3 decoding error: %s", err)
		err = nil
	}

	// volume normalization is applied to the decoded samples, right before
	// they reach the player
	if n > 0 {
		// the loudness of the whole track is measured on the fly: the exact
		// value is remembered for the next plays
		if w.playMeter == nil {
			w.playMeter = newLoudnessMeter(w.decoder.SampleRate())
		}
		w.playMeter.add(dest[:n])

		// the equalizer changes the tone of the track
		w.eq.Apply(dest[:n])

		if guarded := w.normalization.apply(dest[:n]); guarded && !w.duckLogged {
			w.duckLogged = true
			log.Print(log.LVL_INFO, "normalization: track '%s' peak protection lowered the gain to %+.1f dB",
				w.trackTitle, w.normalization.gainDB())
		}
	}

	if w.trackBuffer.IsBuffered() && !w.trackBuffered {
		w.trackBuffered = true
		go w.program.Send(BUFFERING_COMPLETE)
	}

	if w.trackBuffer.IsDone() && !w.trackDone {
		w.trackDone = true
		w.finalizeLoudness()
		w.decoder.Seek(0, io.SeekStart)
		w.trackBuffer.Close()
		go w.program.Send(NEXT)
	} else if !w.trackDone && time.Since(w.lastUpdateTime) > _PROGRESS_UPDATE_PERIOD {
		w.lastUpdateTime = time.Now()
		fraction := ProgressControl(w.trackBuffer.Progress())
		go w.program.Send(fraction)
	}

	return
}

func (w *readWrapper) Seek(offset int64, whence int) (int64, error) {
	w.lastUpdateTime = time.Now()
	if w.decoder == nil {
		return 0, io.EOF
	}
	return w.decoder.Seek(offset, whence)
}

func (w *readWrapper) Length() int64 {
	return w.trackBuffer.Length()
}

func (w *readWrapper) Progress() float64 {
	return w.trackBuffer.Progress()
}

// SampleRate returns the sample rate the track is decoded with.
func (w *readWrapper) SampleRate() int {
	if w.decoder == nil {
		return equalizer.DefaultRate
	}
	return w.decoder.SampleRate()
}

// SetEQ turns the equalizer on or off and sets the gains of its bands.
func (w *readWrapper) SetEQ(enabled bool, gains [equalizer.Bands]float64) {
	if w.eq == nil {
		w.eq = equalizer.New()
	}
	w.eq.Set(enabled, gains)
}

// EQ reports the current state of the equalizer.
func (w *readWrapper) EQ() (enabled bool, gains [equalizer.Bands]float64) {
	if w.eq == nil {
		return false, [equalizer.Bands]float64{}
	}
	return w.eq.Enabled(), w.eq.Gains()
}

// SetEQRate sets the sample rate the equalizer coefficients are computed for.
func (w *readWrapper) SetEQRate(rate int) {
	if w.eq == nil {
		w.eq = equalizer.New()
	}
	w.eq.SetRate(rate)
}
