package playlist

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/phantom-lancer/yatui/ui/style"
)

type ItemDelegate struct {
	programm *tea.Program
	layout   *listLayout
	// hovered is the item under the mouse pointer
	hovered *int
}

func (d ItemDelegate) Height() int {
	return 1
}

func (d ItemDelegate) Spacing() int {
	return 0
}

func (d ItemDelegate) Update(message tea.Msg, m *list.Model) tea.Cmd {
	item, ok := m.SelectedItem().(*Item)
	if !ok {
		return nil
	}

	msg, ok := message.(tea.KeyMsg)
	if !ok {
		return nil
	}

	if (key.Matches(msg, m.KeyMap.CursorUp) || key.Matches(msg, m.KeyMap.CursorDown)) && item.Active {
		go d.programm.Send(item)
	}

	return nil
}

func (d ItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(*Item)
	if !ok {
		return
	}

	name := item.Name
	nameLen := lipgloss.Width(name)
	maxLen := m.Width() - 5
	if nameLen > maxLen {
		name = lipgloss.NewStyle().MaxWidth(maxLen-1).Render(name) + "…"
	}

	var rendered string
	switch {
	case !item.Active && item.Subitem:
		rendered = style.SideBoxSubItemStyle.Render(name)
	case !item.Active:
		rendered = style.SideBoxInactiveItemStyle.Render(name)
	case item.Subitem && index != m.Index():
		rendered = style.SideBoxSubItemStyle.Render(name)
	case item.Subitem:
		rendered = style.SideBoxSelSubItemStyle.Render(name)
	case index != m.Index():
		rendered = style.SideBoxItemStyle.Render(name)
	default:
		rendered = style.SideBoxSelItemStyle.Render(name)
	}

	// the item under the mouse pointer is marked, but the selection is not
	// moved: a click must always play the playlist that is really open
	if d.hovered != nil && *d.hovered == index && index != m.Index() {
		rendered = style.SideBoxHoverItemStyle.Render(name)
	}

	if d.layout != nil {
		d.layout.record(index, lipgloss.Height(rendered))
	}
	fmt.Fprint(w, rendered)
}
