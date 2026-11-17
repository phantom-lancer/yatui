package tracker

import (
	"testing"

	"github.com/phantom-lancer/yatui/api"
)

func TestFindNextLyricIndexUsesFirstTimestampAfterPlaybackPosition(t *testing.T) {
	lines := []api.LyricPair{
		{Timestamp: 1000, Line: "one"},
		{Timestamp: 2000, Line: "two"},
		{Timestamp: 3000, Line: "three"},
	}
	if got := findNextLyricIndex(lines, 1500, true); got != 1 {
		t.Fatalf("index = %d, want 1", got)
	}
	if got := findNextLyricIndex(lines, 3000, true); got != len(lines) {
		t.Fatalf("end index = %d, want %d", got, len(lines))
	}
}

func TestFindNextLyricIndexFallsBackForUnsortedLyrics(t *testing.T) {
	lines := []api.LyricPair{
		{Timestamp: 2000, Line: "later"},
		{Timestamp: 1000, Line: "earlier"},
		{Timestamp: 3000, Line: "last"},
	}
	if got := findNextLyricIndex(lines, 1500, false); got != 0 {
		t.Fatalf("index = %d, want first matching source line at 0", got)
	}
}
