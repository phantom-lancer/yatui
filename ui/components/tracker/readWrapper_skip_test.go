package tracker

import (
	"bytes"
	"io"
	"testing"

	mp3 "github.com/dece2183/go-stream-mp3"

	"github.com/phantom-lancer/yatui/equalizer"
	"github.com/phantom-lancer/yatui/stream"
)

// silentMp3 returns a few MPEG1 layer 3 frames of silence.
func silentMp3(frames int) []byte {
	frame := make([]byte, 417)
	frame[0], frame[1], frame[2], frame[3] = 0xFF, 0xFB, 0x90, 0x64
	return bytes.Repeat(frame, frames)
}

// TestReadWrapperSkipsAKnownLoudness makes sure that a track whose loudness is
// already known plays without the meter: the missing meter used to crash the
// audio thread with a nil dereference.
func TestReadWrapperSkipsAKnownLoudness(t *testing.T) {
	data := silentMp3(10)
	w := &readWrapper{
		// the stream is fully buffered and done: the wrapper must not report
		// the progress anywhere, so no program is needed
		trackBuffered: true,
		trackDone:     true,
		meterSkip:     true,
		normalization: noNormalization,
		eq:            equalizer.New(),
		playMeter:     nil,
	}
	w.trackBuffer = stream.NewBufferedStream(io.NopCloser(bytes.NewReader(data)), int64(len(data)))
	defer w.trackBuffer.Close()

	dec, err := mp3.NewDecoder(w.trackBuffer)
	if err != nil {
		t.Skipf("the synthetic track is not decodable: %s", err)
	}
	w.decoder = dec

	buf := make([]byte, 4096)
	if n, err := w.Read(buf); n == 0 && err != nil {
		t.Fatalf("the first read returned no data: %s", err)
	}
	if w.playMeter != nil {
		t.Fatal("the meter was created for a known loudness")
	}
}
