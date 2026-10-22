package mainpage

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/ui/components/input"
	"github.com/phantom-lancer/yatui/ui/style"
)

// renameLayout is where the rename dialog ended up on the screen, so the
// mouse can be mapped back to the buttons.
type renameLayout struct {
	x, y          int
	width, height int
	buttonRowY    int
	valid         bool
}

// renameView renders the rename dialog with the buttons the user can click.
func (m *Model) renameView() string {
	view := m.inputDialog.View()

	buttons := lipgloss.JoinHorizontal(lipgloss.Center,
		style.ButtonStyle.Padding(0, 1).Render("Применить"),
		"  ",
		style.ButtonStyle.Padding(0, 1).Render("Отмена"),
	)

	view = lipgloss.JoinVertical(lipgloss.Left, view, buttons)

	width := lipgloss.Width(view)
	height := lipgloss.Height(view)
	x := (m.width - width) / 2
	y := (m.height - height) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	m.renameLayout = renameLayout{
		x:          x,
		y:          y,
		width:      width,
		height:     height,
		buttonRowY: y + height - 1,
		valid:      true,
	}

	return view
}

// renameMouse handles the mouse while the rename dialog is open: the buttons
// at the bottom can be clicked, and a click outside of the window cancels it.
func (m *Model) renameMouse(msg tea.MouseMsg) tea.Cmd {
	layout := m.renameLayout
	if !layout.valid {
		return nil
	}

	inside := msg.X >= layout.x && msg.X < layout.x+layout.width &&
		msg.Y >= layout.y && msg.Y < layout.y+layout.height

	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonMiddle {
			break
		}
		if !inside {
			// a click outside of the window cancels it
			m.isRenamePlaylistActive = false
			return tea.ClearScreen
		}
		if msg.Y == layout.buttonRowY {
			centerX := layout.x + layout.width/2
			if msg.X < centerX {
				// apply
				m.isRenamePlaylistActive = false
				return m.renamePlaylistControl(input.APPLY)
			}
			// cancel
			m.isRenamePlaylistActive = false
			return tea.ClearScreen
		}
	}

	return nil
}
