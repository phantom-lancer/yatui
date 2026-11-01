package hotkeys

import (
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

// polledKeys are the function keys the watcher is able to poll. They are
// tracked one by one: a terminal may report F9-F10 itself and still send a
// null byte for F6-F8, so turning the polling off for the whole keyboard
// would kill the keys that only the watcher can deliver.
var polledKeys = [...]tea.KeyType{
	tea.KeyF6, tea.KeyF7, tea.KeyF8, tea.KeyF9, tea.KeyF10, tea.KeyF11, tea.KeyF12,
}

// Tracker tells the watcher which function keys have to be polled.
type Tracker struct {
	polling [len(polledKeys)]atomic.Bool
}

// NewTracker creates a tracker with the polling enabled for every key,
// because some terminals are not able to report function keys.
func NewTracker() *Tracker {
	t := &Tracker{}
	for i := range t.polling {
		t.polling[i].Store(true)
	}
	return t
}

// Start launches the watcher for the given program.
func (t *Tracker) Start(p *tea.Program) {
	Start(p, t.IsActive)
}

// IsActive reports whether the given key has to be polled.
func (t *Tracker) IsActive(key tea.KeyType) bool {
	if index := keyIndex(key); index >= 0 {
		return t.polling[index].Load()
	}
	return false
}

// Stop turns the polling off for every key.
func (t *Tracker) Stop() {
	for i := range t.polling {
		t.polling[i].Store(false)
	}
}

// Observe analyzes an incoming key: a real function key event means the
// terminal supports that key, a null byte means it does not and the key
// has to be polled.
func (t *Tracker) Observe(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyF1, tea.KeyF2, tea.KeyF3, tea.KeyF4, tea.KeyF5,
		tea.KeyF6, tea.KeyF7, tea.KeyF8, tea.KeyF9, tea.KeyF10,
		tea.KeyF11, tea.KeyF12:
		if index := keyIndex(msg.Type); index >= 0 {
			t.polling[index].Store(false)
		}
	case tea.KeyRunes:
		if msg.String() == "\x00" {
			// only F6-F8 are known to arrive as a null byte, so the polling
			// is restored for them alone: the keys the terminal reports
			// itself must not be sent twice
			for _, key := range [...]tea.KeyType{tea.KeyF6, tea.KeyF7, tea.KeyF8} {
				if index := keyIndex(key); index >= 0 {
					t.polling[index].Store(true)
				}
			}
		}
	}
}

// keyIndex returns the place of the key in the polling table, or -1 when
// the key is not polled at all.
func keyIndex(key tea.KeyType) int {
	for i, polled := range polledKeys {
		if polled == key {
			return i
		}
	}
	return -1
}
