package search

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/phantom-lancer/yatui/ui/style"
)

type ItemDelegate struct {
	// rendered collects the rows of the list as they are drawn, so the mouse
	// can be mapped back to a suggestion.
	rendered *[]renderedItem
}

func (d ItemDelegate) Height() int {
	return 2
}

func (d ItemDelegate) Spacing() int {
	return 0
}

func (d ItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d ItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(Item)
	if !ok {
		return
	}

	maxWidth := m.Width() - 3
	if index == m.Index() {
		maxWidth -= 1
	}

	text := string(item)
	textLen := lipgloss.Width(text)
	if textLen > maxWidth {
		text = text[:maxWidth-1] + "…"
	} else if textLen < maxWidth {
		text += strings.Repeat(" ", maxWidth-textLen)
	}

	var stl lipgloss.Style
	if index == m.Index() {
		stl = style.TrackListActiveStyle
	} else {
		stl = style.TrackListStyle
		if index == m.Index()-1 {
			stl = stl.PaddingBottom(0)
		}
		if index%m.Paginator.PerPage == 0 {
			stl = stl.PaddingTop(1)
		}
	}

	rendered := stl.Render(text)
	if d.rendered != nil {
		*d.rendered = append(*d.rendered, renderedItem{
			index:    index,
			height:   lipgloss.Height(rendered),
			contentY: stl.GetBorderTopSize(),
		})
	}

	fmt.Fprint(w, rendered)
}
