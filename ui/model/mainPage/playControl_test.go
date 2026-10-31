package mainpage

import (
	"testing"

	"github.com/phantom-lancer/yatui/ui/components/tracker"
)

// TestRepeatModeCycles checks the order of the repeat modes of the button.
func TestRepeatModeCycles(t *testing.T) {
	m := &Model{}

	for _, want := range []tracker.RepeatMode{
		tracker.RepeatAll,
		tracker.RepeatOne,
		tracker.RepeatOff,
	} {
		m.toggleRepeat()
		if m.repeatMode != want {
			t.Fatalf("the repeat mode is %v, want %v", m.repeatMode, want)
		}
	}
}

// TestRepeatWrapIndex checks where the playback goes when the queue ends.
func TestRepeatWrapIndex(t *testing.T) {
	if got := repeatWrapIndex(5, tracker.RepeatOff); got != -1 {
		t.Errorf("without a repeat the playback must stop, got %d", got)
	}
	if got := repeatWrapIndex(5, tracker.RepeatAll); got != 0 {
		t.Errorf("the whole queue repeats from the start, got %d", got)
	}
	// a single track repeats itself, not the first track of the queue
	if got := repeatWrapIndex(5, tracker.RepeatOne); got != 4 {
		t.Errorf("one track repeats the last one, got %d", got)
	}
}
