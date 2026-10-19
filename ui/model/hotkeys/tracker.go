package hotkeys

import (
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

// Tracker tells the watcher whether the terminal reports
// the function keys on its own.
type Tracker struct {
	polling atomic.Bool
}

// NewTracker creates a tracker with the polling enabled by default,
// because some terminals are not able to report function keys.
func NewTracker() *Tracker {
	t := &Tracker{}
	t.polling.Store(true)
	return t
}

// Start launches the watcher for the given program.
func (t *Tracker) Start(p *tea.Program) {
	Start(p, t.IsActive)
}

// IsActive reports whether the keyboard polling is needed.
func (t *Tracker) IsActive() bool {
	return t.polling.Load()
}

// Stop turns the polling off, e.g. when the terminal turned out
// to report function keys itself.
func (t *Tracker) Stop() {
	t.polling.Store(false)
}

// Observe analyzes an incoming key: a real function key event means
// the terminal supports them, a null byte means it does not.
func (t *Tracker) Observe(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyF1, tea.KeyF2, tea.KeyF3, tea.KeyF4, tea.KeyF5,
		tea.KeyF6, tea.KeyF7, tea.KeyF8, tea.KeyF9, tea.KeyF10,
		tea.KeyF11, tea.KeyF12:
		t.polling.Store(false)
	case tea.KeyRunes:
		if msg.String() == "\x00" {
			t.polling.Store(true)
		}
	}
}
