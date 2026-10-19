//go:build windows

package hotkeys

import (
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
)

// virtual key codes of the player hotkeys
const (
	_VK_F6 = 0x75
	_VK_F7 = 0x76
	_VK_F8 = 0x77
)

var functionKeys = [...]struct {
	vkey int32
	key  tea.KeyType
}{
	{_VK_F6, tea.KeyF6},
	{_VK_F7, tea.KeyF7},
	{_VK_F8, tea.KeyF8},
}

func isKeyDown(vkey int32) bool {
	state, _, _ := procGetAsyncKeyState.Call(uintptr(vkey))
	// the high bit of the result reports that the key is currently down
	return uint16(state)&0x8000 != 0
}

// _POLL_INTERVAL is the keyboard polling period.
// 50ms is short enough to feel instant and keeps the polling
// negligible: three trivial API calls per tick.
const _POLL_INTERVAL = 50 * time.Millisecond

func start(p *tea.Program, isActive func() bool) {
	go func() {
		var pressed [len(functionKeys)]bool
		ticker := time.NewTicker(_POLL_INTERVAL)
		defer ticker.Stop()

		for range ticker.C {
			if isActive != nil && !isActive() {
				// remember the state while the polling is off
				for i, fkey := range functionKeys {
					pressed[i] = isKeyDown(fkey.vkey)
				}
				continue
			}

			for i, fkey := range functionKeys {
				down := isKeyDown(fkey.vkey)
				if down && !pressed[i] {
					p.Send(HotkeyMsg{Key: fkey.key})
				}
				pressed[i] = down
			}
		}
	}()
}
