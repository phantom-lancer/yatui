package tracklist

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dece2183/yamusic-tui/ui/model"
	"github.com/dece2183/yamusic-tui/ui/style"
)

// _ITEM_SPACING must be kept in sync with ItemDelegate.Spacing.
const _ITEM_SPACING = 0

// renderedItem is a list item measured right after it was rendered.
type renderedItem struct {
	index  int
	height int
	// item content offset relative to the rendered item block
	contentY int
	// like indicator position relative to the item block (-1 if none)
	likeX, likeW int
}

// Rect is a rectangular area of the component view.
type Rect struct {
	X, Y, Width, Height int
}

func (r Rect) IsEmpty() bool {
	return r.Width <= 0 || r.Height <= 0
}

func (r Rect) Contains(x, y int) bool {
	if r.IsEmpty() {
		return false
	}
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// itemRow is a position of a rendered item inside the component view.
type itemRow struct {
	index  int
	y      int
	height int
	like   Rect
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
func (l *listLayout) record(index, height, contentY, likeX, likeW int) {
	l.rendered = append(l.rendered, renderedItem{
		index:    index,
		height:   height,
		contentY: contentY,
		likeX:    likeX,
		likeW:    likeW,
	})
}

// build turns measured items into rows placed inside the view.
func (l *listLayout) build(originX, originY, spacing int) {
	l.originX = originX
	l.originY = originY
	l.rows = make([]itemRow, 0, len(l.rendered))
	y := l.originY
	for _, item := range l.rendered {
		row := itemRow{index: item.index, y: y, height: item.height}
		if item.likeW > 0 {
			row.like = Rect{
				X:      originX + item.likeX,
				Y:      y + item.contentY,
				Width:  item.likeW,
				Height: 1,
			}
		}
		l.rows = append(l.rows, row)
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

// ItemAt returns the index of the item displayed at the given component
// view coordinates. Returns -1 if there is no item there.
func (m *Model) ItemAt(x, y int) int {
	row, ok := m.layout.rowAt(x, y)
	if !ok {
		return -1
	}
	return row.index
}

// LikeAt returns the index of the item whose like indicator is placed at
// the given component view coordinates. Returns -1 if there is none.
func (m *Model) LikeAt(x, y int) int {
	row, ok := m.layout.rowAt(x, y)
	if !ok {
		return -1
	}
	if row.like.Contains(x, y) {
		return row.index
	}
	return -1
}

// CursorUp moves the cursor to the previous item.
func (m *Model) CursorUp() tea.Cmd {
	if len(m.list.Items()) == 0 {
		return nil
	}
	m.list.CursorUp()
	return model.Cmd(CURSOR_UP)
}

// CursorDown moves the cursor to the next item.
func (m *Model) CursorDown() tea.Cmd {
	if len(m.list.Items()) == 0 {
		return nil
	}
	m.list.CursorDown()
	return model.Cmd(CURSOR_DOWN)
}

// updateLayout measures list sections that are placed above the items.
// resetLayout must be called before the list is rendered,
// buildLayout right after it.
func (m *Model) resetLayout() {
	m.layout.reset()
}

func (m *Model) buildLayout() {
	originX := style.TrackBoxStyle.GetBorderLeftSize() + style.TrackBoxStyle.GetPaddingLeft()
	originY := style.TrackBoxStyle.GetBorderTopSize() + style.TrackBoxStyle.GetPaddingTop()

	if m.list.ShowTitle() {
		originY += lipgloss.Height(m.list.Styles.TitleBar.Render("x"))
	}
	if m.list.ShowStatusBar() {
		originY += lipgloss.Height(m.list.Styles.StatusBar.Render("x"))
	}

	m.layout.build(originX, originY, _ITEM_SPACING)
}
