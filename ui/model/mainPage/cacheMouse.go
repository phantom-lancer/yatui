package mainpage

import (
	tea "github.com/charmbracelet/bubbletea"
)

// cacheMouse handles the mouse while the cache manager is open: the tracks
// can be pointed at and played.
func (m *Model) cacheMouse(msg tea.MouseMsg) tea.Cmd {
	if m.cache == nil {
		return nil
	}
	layout := m.cache.layout

	switch msg.Action {
	case tea.MouseActionMotion:
		if msg.Button != tea.MouseButtonNone {
			break
		}
		if row, ok := layout.rowAt(msg.X, msg.Y); ok {
			index := m.cache.offset + row
			if index >= 0 && index < len(m.cache.tracks) && index != m.cache.index {
				m.cache.index = index
				m.cache.ensureVisible(m.height-16, true)
			}
		}

	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonMiddle {
			break
		}
		row, ok := layout.rowAt(msg.X, msg.Y)
		if !ok {
			// a click outside of the window closes it
			m.isCacheActive = false
			m.cache = nil
			return tea.ClearScreen
		}

		index := m.cache.offset + row
		if index < 0 || index >= len(m.cache.tracks) {
			return nil
		}
		m.cache.index = index
		cmd := m.playCachedTrack(index)
		m.isCacheActive = false
		m.cache = nil
		return tea.Batch(tea.ClearScreen, cmd)
	}

	return nil
}

// rowAt returns the index of the row under the given screen coordinates.
func (l cacheLayout) rowAt(x, y int) (int, bool) {
	if !l.valid || x < l.x || x >= l.x+l.width || y < l.rowY {
		return 0, false
	}
	if row := y - l.rowY; row < l.rows {
		return row, true
	}
	return 0, false
}
