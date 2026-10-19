package tracker

import (
	"io"
	"math"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	mp3 "github.com/dece2183/go-stream-mp3"
	"github.com/dece2183/yamusic-tui/api"
	"github.com/dece2183/yamusic-tui/log"
	"github.com/dece2183/yamusic-tui/stream"
)

const (
	_PROGRESS_UPDATE_PERIOD = 850 * time.Millisecond

	// _LOUDNESS_ANALYSIS_SECONDS is how much of a track is decoded to measure
	// its loudness. Six seconds are enough for a stable estimate and cost a
	// fraction of a second of CPU once per track.
	_LOUDNESS_ANALYSIS_SECONDS = 6
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
	trackTitle     string
	// gainDB keeps the applied correction as the bits of a float64, so the UI
	// can read it without touching the audio thread.
	gainDB uint64
}

func (w *readWrapper) NewReader(reader *stream.BufferedStream, track *api.Track) {
	var err error

	w.trackBuffered = false
	w.trackDone = false
	w.trackBuffer = reader
	if track != nil {
		w.trackTitle = track.Title
	}

	w.normalization, w.measurePending = newNormalization(track)
	atomic.StoreUint64(&w.gainDB, math.Float64bits(w.normalization.gainDB()))

	w.decoder, err = mp3.NewDecoder(w.trackBuffer)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to create mp3 decoder: %s", err)
		return
	}

	w.lastUpdateTime = time.Now()
}

// measureLoudness decodes the beginning of the track to find out how loud it
// is, rewinds the stream and hands the correction to the audio callback. It
// runs on the player thread, before the first samples are played.
func (w *readWrapper) measureLoudness() {
	if w.decoder == nil || w.trackBuffer == nil {
		return
	}

	meter := newLoudnessMeter(w.decoder.SampleRate())
	buffer := make([]byte, 32*1024)
	wanted := meter.sampleRate * 4 * _LOUDNESS_ANALYSIS_SECONDS

	for read := 0; read < wanted; {
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
	if w.normalization.scale() {
		log.Print(log.LVL_INFO, "normalization: track '%s' corrected by %.1f dB", w.trackTitle, w.normalization.gainDB())
	}
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
		w.normalization.apply(dest[:n])
	}

	if w.trackBuffer.IsBuffered() && !w.trackBuffered {
		w.trackBuffered = true
		go w.program.Send(BUFFERING_COMPLETE)
	}

	if w.trackBuffer.IsDone() && !w.trackDone {
		w.trackDone = true
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
	return w.decoder.Seek(offset, whence)
}

func (w *readWrapper) Length() int64 {
	return w.trackBuffer.Length()
}

func (w *readWrapper) Progress() float64 {
	return w.trackBuffer.Progress()
}
