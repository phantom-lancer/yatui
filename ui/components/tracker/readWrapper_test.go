package tracker

import (
	"bytes"
	"io"
	"testing"

	"github.com/phantom-lancer/yatui/stream"
)

func TestPCMLengthUsesDecodedStereoS16Bytes(t *testing.T) {
	const sampleRate = 44100
	const durationMs = 180000
	want := int64(180000 * sampleRate * 4 / 1000)
	if got := pcmLengthBytes(durationMs, sampleRate); got != want {
		t.Fatalf("PCM length = %d, want %d", got, want)
	}
	if got := pcmLengthBytes(0, sampleRate); got != 0 {
		t.Fatalf("zero duration length = %d, want 0", got)
	}
	if got := pcmLengthBytes(durationMs, 0); got != 0 {
		t.Fatalf("zero sample rate length = %d, want 0", got)
	}
}

// TestReadWrapperSurvivesABrokenStream makes sure that a track with a broken
// download does not crash the player: the decoder is missing then and the
// player must get the end of the track.
func TestReadWrapperSurvivesABrokenStream(t *testing.T) {
	w := &readWrapper{}

	// nothing at all
	buf := make([]byte, 1024)
	if n, err := w.Read(buf); n != 0 || err == nil {
		t.Fatalf("an empty wrapper returned %d, %v", n, err)
	}

	// a stream without a decoder
	w.trackBuffer = stream.NewBufferedStream(io.NopCloser(bytes.NewReader(nil)), 0)
	defer w.trackBuffer.Close()

	if n, err := w.Read(buf); n != 0 || err == nil {
		t.Fatalf("a broken stream returned %d, %v", n, err)
	}

	// seeking and closing must not panic either
	w.Seek(0, io.SeekStart)
	w.Close()
}
