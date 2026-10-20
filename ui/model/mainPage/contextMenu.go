package mainpage

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/phantom-lancer/yatui/ui/style"
)

// contextMenu is a small overlay shown on right click.
type contextMenu struct {
	x, y     int
	items    []contextMenuItem
	index    int
	trackIdx int
	plIdx    int
	// maxHeight is the height of the screen: a long menu is cut off, so it
	// never reaches beyond the screen.
	maxHeight int
}

type contextMenuItem struct {
	label  string
	action func(m *Model) tea.Cmd
}

func (cm *contextMenu) up() {
	if cm.index > 0 {
		cm.index--
	}
}

func (cm *contextMenu) down() {
	if cm.index < len(cm.items)-1 {
		cm.index++
	}
}

// hover highlights the item under the cursor.
func (cm *contextMenu) hover(x, y int) {
	if idx := cm.itemAt(x, y); idx >= 0 {
		cm.index = idx
	}
}

// size returns the menu size in cells: width and height of the box
// including its border.
func (cm *contextMenu) size() (int, int) {
	labelWidth := 0
	for _, item := range cm.items {
		if w := ansi.StringWidth(item.label); w > labelWidth {
			labelWidth = w
		}
	}

	// border (2) + left padding (1) + right padding (1) + the widest label
	width := labelWidth + 4
	if width < 20 {
		width = 20
	}

	// border top + items + border bottom
	height := len(cm.items) + 2
	if cm.maxHeight > 0 && height > cm.maxHeight {
		height = cm.maxHeight
	}

	return width, height
}

func (cm *contextMenu) contains(x, y int) bool {
	w, h := cm.size()
	return x >= cm.x && x < cm.x+w && y >= cm.y && y < cm.y+h
}

// itemAt returns the index of the item at the given screen coordinates,
// or -1 if there is none (borders and empty padding included).
func (cm *contextMenu) itemAt(x, y int) int {
	if !cm.contains(x, y) {
		return -1
	}

	// the first item is placed right below the top border
	index := y - cm.y - 1
	if index < 0 || index >= len(cm.items) {
		return -1
	}
	return index
}

func (cm *contextMenu) selectIndex(m *Model, index int) tea.Cmd {
	if index < 0 || index >= len(cm.items) {
		return nil
	}
	cm.index = index
	return cm.selectItem(m)
}

func (cm *contextMenu) selectItem(m *Model) tea.Cmd {
	if cm.index < 0 || cm.index >= len(cm.items) {
		return nil
	}
	return cm.items[cm.index].action(m)
}

// render draws the menu over the given view without breaking it:
// the underlying lines are cut by cells, not by bytes, so the ANSI
// sequences of the original view stay intact.
func (cm *contextMenu) render(base string) string {
	lines := strings.Split(base, "\n")

	// the view width is the widest line, not the whole string
	baseWidth := 0
	for _, line := range lines {
		if w := ansi.StringWidth(line); w > baseWidth {
			baseWidth = w
		}
	}
	baseHeight := len(lines)

	width, height := cm.size()

	// keep the menu inside the view
	x := cm.x
	if x+width > baseWidth {
		x = baseWidth - width
	}
	if x < 0 {
		x = 0
	}

	y := cm.y
	if y+height > baseHeight {
		y = baseHeight - height
	}
	if y < 0 {
		y = 0
	}

	// remember where the menu has actually been drawn
	cm.x, cm.y = x, y

	// the menu box is drawn in the local coordinates
	menu := cm.draw(width, height)

	for i, menuLine := range menu {
		row := y + i
		if row >= len(lines) {
			break
		}
		lines[row] = overlay(lines[row], menuLine, x)
	}

	return strings.Join(lines, "\n")
}

// draw renders the menu box into its own lines.
func (cm *contextMenu) draw(width, height int) []string {
	const (
		topLeft     = "╭"
		topRight    = "╮"
		bottomLeft  = "╰"
		bottomRight = "╯"
		horizontal  = "─"
		vertical    = "│"
	)

	// inner area of the box
	inner := width - 2

	// the selected item is painted over the whole inner area
	selected := style.ActiveButtonStyle.Margin(0).Padding(0, 0).Width(inner).Height(1)
	plain := lipgloss.NewStyle().Margin(0).Padding(0, 0).Width(inner).Height(1)

	lines := make([]string, 0, height)
	lines = append(lines, topLeft+strings.Repeat(horizontal, inner)+topRight)

	for i, entry := range cm.items {
		itemStyle := plain
		if i == cm.index {
			itemStyle = selected
		}
		lines = append(lines, vertical+itemStyle.Render(entry.label)+vertical)
	}

	lines = append(lines, bottomLeft+strings.Repeat(horizontal, inner)+bottomRight)

	return lines
}

// overlay writes the menu line over the given cells of the base line.
func overlay(base, menu string, x int) string {
	menuWidth := ansi.StringWidth(menu)

	// widen the line if the menu hangs over its right edge
	width := ansi.StringWidth(base)
	if width < x+menuWidth {
		base += strings.Repeat(" ", x+menuWidth-width)
		width = x + menuWidth
	}

	prefix := ansi.Cut(base, 0, x)
	suffix := ansi.Cut(base, x+menuWidth, width)

	// The menu box is opaque: it keeps the accent border and covers
	// the cells underneath, so nothing of the original line shows
	// through. The rest of the line keeps its original styling,
	// the reset prevents the colors from bleeding into it.
	return prefix + opaque(menu) + "\x1b[0m" + suffix
}

// opaque returns the menu line with a background applied,
// so that the content underneath is fully covered.
func opaque(menu string) string {
	return lipgloss.NewStyle().Background(style.BackgroundColor).Render(menu)
}
