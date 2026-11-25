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
	program          *tea.Program
	decoder          *mp3.Decoder
	trackBuffer      *stream.BufferedStream
	deferEnded       bool
	playbackProgress uint64
	decodedPosition  int64
	durationMs       int
	trackBuffered    bool
	trackDone        bool
	lastUpdateTime   time.Time
	normalization    normalization
	measurePending   bool
	// meterSkip is set when the loudness of the track is already known or not
	// needed: the whole track must not be measured again while it plays.
	meterSkip  bool
	duckLogged bool
	trackID    string
	trackTitle string
	// playMeter measures the whole track while it plays: the result is exact
	// and is remembered for the next plays.
	playMeter *loudnessMeter
	// gainDB keeps the applied correction as the bits of a float64, so the UI
	// can read it without touching the audio thread.
	gainDB uint64
	// eq applies the classic ten band equalizer to the decoded PCM.
	eq *equalizer.Processor
	// graphic is the fifteen band equalizer of the graphic view.
	graphic *equalizer.GraphicProcessor
	// graphicMode selects which engine is applied. Both keep their own
	// gains and history, so switching the view neither loses the settings
	// nor feeds the samples through two cascades.
	graphicMode bool
	// fastNormalization is read by the audio thread; it is stored as an
	// atomic word so the UI can change it without a lock on the audio path.
	fastNormalization uint32
	// normalizationOn is the snapshot of the normalization setting. The
	// audio path never reads the mutable config.
	normalizationOn uint32
	// amplitude is the peak of the last decoded buffer as the bits of a
	// float32. The visualizer reads it from the UI thread without a lock.
	amplitude uint32
	// vizOn is the snapshot of the visualizer setting: with the visualizer
	// off the audio thread does not scan the buffer at all.
	vizOn uint32
}

func (w *readWrapper) NewReader(reader *stream.BufferedStream, track *api.Track) error {
	var err error

	w.trackBuffered = false
	w.trackDone = false
	w.deferEnded = false
	atomic.StoreUint64(&w.playbackProgress, math.Float64bits(0))
	atomic.StoreInt64(&w.decodedPosition, 0)
	w.durationMs = 0
	if track != nil {
		w.durationMs = track.DurationMs
	}
	w.trackBuffer = reader
	w.playMeter = nil
	w.duckLogged = false
	w.trackID = ""
	if track != nil {
		w.trackID = track.Id
		w.trackTitle = track.Title
	}

	w.normalization, w.measurePending = newNormalization(track, atomic.LoadUint32(&w.normalizationOn) != 0)

	if w.eq == nil {
		w.eq = equalizer.New()
	} else {
		w.eq.Reset()
	}
	if w.graphic == nil {
		w.graphic = equalizer.NewGraphic()
	} else {
		w.graphic.Reset()
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

	// a known loudness needs no measurement: the whole track would otherwise
	// be fed to the meter on every play for an answer that is already stored
	w.meterSkip = !w.measurePending

	atomic.StoreUint64(&w.gainDB, math.Float64bits(w.normalization.gainDB()))
	atomic.StoreUint32(&w.amplitude, 0)

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

	meter, _ := analyzeHead(w.decoder, w.trackTitle, atomic.LoadUint32(&w.fastNormalization) != 0)

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
	atomic.StoreInt64(&w.decodedPosition, 0)

	w.normalization = meter.normalization()
	atomic.StoreUint64(&w.gainDB, math.Float64bits(w.normalization.gainDB()))
}

// analyzeHead decodes the head of a track and returns the meter with the
// measured loudness and the number of seconds it covers. The stream is left
// right after the decoded part.
func analyzeHead(decoder *mp3.Decoder, title string, fast bool) (*loudnessMeter, int) {
	meter := newLoudnessMeter(decoder.SampleRate())
	bytesPerSecond := meter.sampleRate * 4
	buffer := make([]byte, 32*1024)
	wanted := bytesPerSecond * _LOUDNESS_ANALYSIS_SECONDS
	// the quick measurement reads the first second only: cheap, and close
	// enough thanks to the per-track correction measured while playing
	if fast {
		wanted = bytesPerSecond
	}
	read := 0

	for read < wanted {
		n, err := decoder.Read(buffer)
		if n > 0 {
			meter.add(buffer[:n])
			read += n
		}
		if err != nil {
			break
		}
	}

	seconds := read / bytesPerSecond
	if loudness := meter.integratedLoudness(); math.IsInf(loudness, -1) {
		log.Print(log.LVL_INFO, "normalization: track '%s' is silent, played as is", title)
	} else {
		log.Print(log.LVL_INFO, "normalization: track '%s' %.1f LUFS in the first %d s (peak %.2f), gain %+.1f dB",
			title, loudness, seconds, meter.peak, meter.normalization().gainDB())
	}

	return meter, seconds
}

// MeasureTrack measures the loudness of the head of a track and remembers it
// before the track reaches the player. It runs in the download goroutine: the
// first play of a track used to decode ten seconds of audio on the audio
// thread, which froze the window for seconds when the track was switched.
func MeasureTrack(reader *stream.BufferedStream, track *api.Track, fast, enabled bool) {
	if reader == nil || track == nil {
		return
	}
	if _, pending := newNormalization(track, enabled); !pending {
		return
	}
	if _, ok := lookupLoudness(track.Id); ok {
		return
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return
	}

	decoder, err := mp3.NewDecoder(reader)
	if err != nil {
		log.Print(log.LVL_WARNIGN, "normalization: unable to decode track '%s': %s", track.Title, err)
		return
	}

	meter, _ := analyzeHead(decoder, track.Title, fast)

	// the analysis has consumed the head of the stream: the player starts
	// from the very first frame again
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		log.Print(log.LVL_WARNIGN, "normalization: unable to rewind track '%s': %s", track.Title, err)
		return
	}

	if loudness := meter.integratedLoudness(); !math.IsInf(loudness, -1) {
		storeLoudness(track.Id, loudnessEntry{Loudness: loudness, Peak: meter.peak})
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
	if n > 0 {
		atomic.AddInt64(&w.decodedPosition, int64(n))
	}
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
		if !w.meterSkip {
			if w.playMeter == nil {
				w.playMeter = newLoudnessMeter(w.decoder.SampleRate())
			}
			w.playMeter.add(dest[:n])
		}

		// the equalizer changes the tone of the track
		if w.graphicMode {
			w.graphic.Apply(dest[:n])
		} else {
			w.eq.Apply(dest[:n])
		}

		if guarded := w.normalization.apply(dest[:n]); guarded && !w.duckLogged {
			w.duckLogged = true
			log.Print(log.LVL_INFO, "normalization: track '%s' peak protection lowered the gain to %+.1f dB",
				w.trackTitle, w.normalization.gainDB())
		}

		// the visualizer follows the music, not the decode loop: the peak of
		// the buffer is enough and costs one pass over the samples. When the
		// visualizer is off, even that pass is skipped.
		if atomic.LoadUint32(&w.vizOn) != 0 {
			atomic.StoreUint32(&w.amplitude, math.Float32bits(float32(pcmLevel(dest[:n]))))
		} else {
			atomic.StoreUint32(&w.amplitude, 0)
		}
	}

	if w.trackBuffer.IsBuffered() && !w.trackBuffered {
		w.trackBuffered = true
		go w.program.Send(BUFFERING_COMPLETE)
	}

	if w.trackBuffer.IsDone() && !w.trackDone {
		w.trackDone = true
		w.finalizeLoudness()
		if !w.deferEnded {
			w.trackBuffer.Close()
			w.sendEnded()
		}
	} else if !w.trackDone && !w.deferEnded && time.Since(w.lastUpdateTime) > _PROGRESS_UPDATE_PERIOD {
		w.lastUpdateTime = time.Now()
		w.sendProgress(w.trackBuffer.Progress())
	}

	return
}

func (w *readWrapper) Seek(offset int64, whence int) (int64, error) {
	w.lastUpdateTime = time.Now()
	if w.decoder == nil {
		return 0, io.EOF
	}
	pos, err := w.decoder.Seek(offset, whence)
	if err == nil {
		w.trackDone = false
		atomic.StoreInt64(&w.decodedPosition, pos)
		if w.deferEnded {
			atomic.StoreUint64(&w.playbackProgress, math.Float64bits(w.SourceProgress()))
		}
	}
	return pos, err
}

func pcmLengthBytes(durationMs, sampleRate int) int64 {
	if durationMs <= 0 || sampleRate <= 0 {
		return 0
	}
	// MP3 decoder offsets and output positions are PCM bytes. Decoder output
	// is always stereo S16, so each sample frame occupies 4 bytes.
	return int64(float64(durationMs) * float64(sampleRate) * 4 / 1000)
}

func (w *readWrapper) Length() int64 {
	if w.decoder != nil {
		return pcmLengthBytes(w.durationMs, w.decoder.SampleRate())
	}
	if w.trackBuffer == nil {
		return 0
	}
	return w.trackBuffer.Length()
}

// SourceProgress estimates decoded PCM position from track duration.
// Calling Decoder.Length can require scanning an MP3 without a Xing header,
// so use the duration already supplied by the API instead.
func (w *readWrapper) SourceProgress() float64 {
	if w.decoder == nil {
		if w.trackBuffer == nil {
			return 0
		}
		return w.trackBuffer.Progress()
	}
	totalPCMBytes := float64(pcmLengthBytes(w.durationMs, w.decoder.SampleRate()))
	if totalPCMBytes <= 0 {
		return 0
	}
	return clampPCMProgress(float64(atomic.LoadInt64(&w.decodedPosition)) / totalPCMBytes)
}

// pcmLevel returns the loudest sample of a stereo S16LE buffer as 0..1. It
// runs on the audio thread, so it does not allocate and stops at the first
// sample of a full scale peak.
func pcmLevel(pcm []byte) float64 {
	peak := 0
	for i := 0; i+1 < len(pcm); i += 2 {
		sample := int(int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8))
		if sample < 0 {
			sample = -sample
		}
		if sample > peak {
			peak = sample
			if peak >= 32767 {
				return 1
			}
		}
	}
	return float64(peak) / 32768
}

func clampPCMProgress(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func (w *readWrapper) Progress() float64 {
	if w.deferEnded {
		return math.Float64frombits(atomic.LoadUint64(&w.playbackProgress))
	}
	return w.SourceProgress()
}

func (w *readWrapper) setProgress(fraction float64) {
	atomic.StoreUint64(&w.playbackProgress, math.Float64bits(fraction))
}

func (w *readWrapper) sendProgress(fraction float64) {
	w.setProgress(fraction)
	if w.program != nil {
		go w.program.Send(ProgressControl(fraction))
	}
}

// sendReady starts the normal volume fade only after the PCM worker has
// primed enough audio to survive a short CPU scheduling interruption.
func (w *readWrapper) sendReady() {
	if w.program != nil {
		go w.program.Send(VolumeFadeStartMsg{})
	}
}

func (w *readWrapper) sendSeekError(err error) {
	if err != nil {
		log.Print(log.LVL_WARNIGN, "audio seek failed: %s", err)
	}
}

func (w *readWrapper) sendEnded() {
	if w.program != nil {
		go w.program.Send(ENDED)
	}
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
	if w.graphic == nil {
		w.graphic = equalizer.NewGraphic()
	}
	w.eq.SetRate(rate)
	w.graphic.SetRate(rate)
}

// SetGraphicEQ turns the fifteen band equalizer on or off.
func (w *readWrapper) SetGraphicEQ(enabled bool, gains [equalizer.GraphicBands]float64) {
	if w.graphic == nil {
		w.graphic = equalizer.NewGraphic()
	}
	w.graphic.Set(enabled, gains)
}

// GraphicEQ reports the state of the fifteen band equalizer.
func (w *readWrapper) GraphicEQ() (enabled bool, gains [equalizer.GraphicBands]float64) {
	if w.graphic == nil {
		return false, [equalizer.GraphicBands]float64{}
	}
	return w.graphic.Enabled(), w.graphic.Gains()
}

// SetEQMode selects the ten or the fifteen band engine.
func (w *readWrapper) SetEQMode(graphic bool) {
	w.graphicMode = graphic
}

// EQMode reports whether the graphic engine is selected.
func (w *readWrapper) EQMode() bool {
	return w.graphicMode
}

// SetVisualizer tells the audio thread whether the level is needed.
func (w *readWrapper) SetVisualizer(enabled bool) {
	if enabled {
		atomic.StoreUint32(&w.vizOn, 1)
	} else {
		atomic.StoreUint32(&w.vizOn, 0)
	}
}

// SetNormalization enables or disables the normalized playback. The value is
// a snapshot: the audio thread reads the atomic word, not the config.
func (w *readWrapper) SetNormalization(enabled bool) {
	if enabled {
		atomic.StoreUint32(&w.normalizationOn, 1)
	} else {
		atomic.StoreUint32(&w.normalizationOn, 0)
	}
}

// SetFastNormalization selects the one second loudness measurement.
func (w *readWrapper) SetFastNormalization(fast bool) {
	if fast {
		atomic.StoreUint32(&w.fastNormalization, 1)
	} else {
		atomic.StoreUint32(&w.fastNormalization, 0)
	}
}
