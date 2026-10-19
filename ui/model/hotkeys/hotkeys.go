package hotkeys

import (
	tea "github.com/charmbracelet/bubbletea"
)

// HotkeyMsg is a function key detected by the keyboard watcher.
// It is a separate type on purpose: a key event received from the
// terminal disables the watcher, its own events must not do that.
type HotkeyMsg struct {
	Key tea.KeyType
}

// String returns the key name, e.g. "f6".
func (m HotkeyMsg) String() string {
	return tea.KeyMsg{Type: m.Key}.String()
}

// Start launches the function keys watcher.
//
// Some terminals report F6-F8 as a null byte instead of a key event,
// so they cannot be handled by the keyboard input stream at all.
// The watcher polls the keyboard state and sends the missing events
// to the program itself.
//
// isActive reports whether the polling is needed: it is turned off
// as soon as the terminal turns out to deliver function keys itself.
func Start(p *tea.Program, isActive func() bool) {
	start(p, isActive)
}
