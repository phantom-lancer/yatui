package mainpage

import (
	"bytes"
	"io"
	"testing"

	"github.com/phantom-lancer/yatui/stream"
)

func TestTakePreloadedTransfersMatchingTrack(t *testing.T) {
	m := &Model{}
	buffer := stream.NewBufferedStream(io.NopCloser(bytes.NewReader([]byte("track"))), 5)
	m.preload.item = &preloadedTrack{id: "track-id", buffer: buffer}

	got := m.takePreloaded("track-id")
	if got != buffer {
		t.Fatal("matching preloaded stream was not transferred")
	}
	if m.preload.item != nil {
		t.Fatal("transferred stream remained in preload state")
	}
	if err := got.Close(); err != nil {
		t.Fatalf("close transferred stream: %v", err)
	}
}

func TestTakePreloadedClosesMismatchedTrack(t *testing.T) {
	m := &Model{}
	buffer := stream.NewBufferedStream(io.NopCloser(bytes.NewReader([]byte("track"))), 5)
	m.preload.item = &preloadedTrack{id: "old-track", buffer: buffer}

	if got := m.takePreloaded("new-track"); got != nil {
		t.Fatal("mismatched stream was returned")
	}
	if m.preload.item != nil {
		t.Fatal("mismatched stream remained in preload state")
	}
	if _, err := buffer.Read(make([]byte, 1)); err == nil {
		t.Fatal("mismatched stream was not closed")
	}
}

func TestClearPreloadInvalidatesPendingGeneration(t *testing.T) {
	m := &Model{}
	before := m.preload.generation
	m.clearPreload()
	if m.preload.generation != before+1 {
		t.Fatalf("generation = %d, want %d", m.preload.generation, before+1)
	}
}
