package hotkeys

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestPollingIsTurnedOffPerKey makes sure a function key the terminal reports
// itself does not turn the polling off for the keys only the watcher can
// deliver.
func TestPollingIsTurnedOffPerKey(t *testing.T) {
	tracker := NewTracker()

	tracker.Observe(tea.KeyMsg{Type: tea.KeyF9})
	if tracker.IsActive(tea.KeyF9) {
		t.Error("F9 is reported by the terminal, it must not be polled")
	}
	for _, key := range []tea.KeyType{tea.KeyF6, tea.KeyF7, tea.KeyF8, tea.KeyF10} {
		if !tracker.IsActive(key) {
			t.Errorf("%v has to stay polled", key)
		}
	}

	// a terminal that sends F6-F8 as a null byte gets the polling back
	tracker.Observe(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0}})
	for _, key := range []tea.KeyType{tea.KeyF6, tea.KeyF7, tea.KeyF8} {
		if !tracker.IsActive(key) {
			t.Errorf("%v has to be polled after a null byte", key)
		}
	}
	if tracker.IsActive(tea.KeyF9) {
		t.Error("a key delivered by the terminal must not be polled twice")
	}

	tracker.Stop()
	if tracker.IsActive(tea.KeyF6) {
		t.Error("Stop must turn the polling off")
	}
}
