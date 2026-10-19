package playlist

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dece2183/yamusic-tui/ui/model"
	"github.com/dece2183/yamusic-tui/ui/style"
)

// renderedItem is a list item measured right after it was rendered.
type renderedItem struct {
	index  int
	height int
}

// _ITEM_SPACING must be kept in sync with ItemDelegate.Spacing.
const _ITEM_SPACING = 0

// itemRow is a position of a rendered item inside the component view.
type itemRow struct {
	index  int
	y      int
	height int
}

// listLayout keeps mouse related geometry of the last rendered view.
type listLayout struct {
	originX, originY int
	rendered         []renderedItem
	rows             []itemRow
}

func (l *listLayout) reset() {
	l.rendered = nil
	l.rows = nil
	l.originX = 0
	l.originY = 0
}

// record is called by the delegate on every item render.
func (l *listLayout) record(index, height int) {
	l.rendered = append(l.rendered, renderedItem{index: index, height: height})
}

// build turns measured items into rows placed inside the view.
func (l *listLayout) build(spacing int) {
	l.rows = make([]itemRow, 0, len(l.rendered))
	y := l.originY
	for _, item := range l.rendered {
		l.rows = append(l.rows, itemRow{index: item.index, y: y, height: item.height})
		// bubbles list joins items with (spacing + 1) newlines,
		// the first one just terminates the previous item line
		y += item.height + spacing
	}
}

func (l *listLayout) rowAt(x, y int) (itemRow, bool) {
	if x < l.originX {
		return itemRow{}, false
	}
	for _, row := range l.rows {
		if row.height > 0 && y >= row.y && y < row.y+row.height {
			return row, true
		}
	}
	return itemRow{}, false
}

// ItemAt returns the index of the list item displayed at the given
// component view coordinates. Returns -1 if there is no item there.
func (m *Model) ItemAt(x, y int) int {
	row, ok := m.layout.rowAt(x, y)
	if !ok {
		return -1
	}
	return row.index
}

// ClickItem selects the item at the given index.
// Returns false if there is nothing to select there.
func (m *Model) ClickItem(index int) bool {
	items := m.list.Items()
	if index < 0 || index >= len(items) {
		return false
	}
	item, ok := items[index].(*Item)
	if !ok || !item.Active {
		return false
	}
	m.list.Select(index)
	return true
}

// CursorUp moves the cursor to the previous selectable item.
func (m *Model) CursorUp() tea.Cmd {
	if len(m.list.Items()) == 0 {
		return nil
	}

	m.list.CursorUp()
	for len(m.list.Items()) > 0 && m.list.Index() > 0 && !m.list.SelectedItem().(*Item).Active {
		m.list.CursorUp()
	}

	return model.Cmd(CURSOR_UP)
}

// CursorDown moves the cursor to the next selectable item.
func (m *Model) CursorDown() tea.Cmd {
	if len(m.list.Items()) == 0 {
		return nil
	}

	m.list.CursorDown()
	for m.list.Index() < len(m.list.Items())-1 && !m.list.SelectedItem().(*Item).Active {
		m.list.CursorDown()
	}

	return model.Cmd(CURSOR_DOWN)
}

// updateLayout measures list sections that are placed above the items.
// resetLayout must be called before the list is rendered,
// buildLayout right after it.
func (m *Model) resetLayout() {
	m.layout.reset()
}

func (m *Model) buildLayout() {
	m.layout.originX = style.SideBoxStyle.GetBorderLeftSize() + style.SideBoxStyle.GetPaddingLeft()
	m.layout.originY = style.SideBoxStyle.GetBorderTopSize() + style.SideBoxStyle.GetPaddingTop()

	if m.list.ShowTitle() {
		m.layout.originY += lipgloss.Height(m.list.Styles.TitleBar.Render("x"))
	}
	if m.list.ShowStatusBar() {
		m.layout.originY += lipgloss.Height(m.list.Styles.StatusBar.Render("x"))
	}

	m.layout.build(_ITEM_SPACING)
}
