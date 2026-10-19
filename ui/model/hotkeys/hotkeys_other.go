//go:build !windows

package hotkeys

import (
	tea "github.com/charmbracelet/bubbletea"
)

func start(p *tea.Program, isActive func() bool) {}
