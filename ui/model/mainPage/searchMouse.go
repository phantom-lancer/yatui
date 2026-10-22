package mainpage

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/ui/components/search"
	"github.com/phantom-lancer/yatui/ui/model"
)

// searchMouse handles the mouse while the search dialog is open: the
// suggestions can be pointed at and picked.
func (m *Model) searchMouse(msg tea.MouseMsg) tea.Cmd {
	if m.searchDialog == nil {
		return nil
	}

	// the dialog is in the middle of the screen
	view := m.searchDialog.View()
	x := (m.width - lipgloss.Width(view)) / 2
	y := (m.height - lipgloss.Height(view)) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	index := m.searchDialog.ItemAt(msg.X-x, msg.Y-y)

	switch msg.Action {
	case tea.MouseActionMotion:
		if msg.Button != tea.MouseButtonNone {
			break
		}
		m.searchDialog.Hover(index)

	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonMiddle {
			break
		}
		if index < 0 {
			break
		}
		if !m.searchDialog.Pick(index) {
			break
		}
		return model.Cmd(search.SELECT)
	}

	return nil
}
