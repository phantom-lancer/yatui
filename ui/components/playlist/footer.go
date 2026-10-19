package playlist

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

// SetFooter sets the block rendered under the list. An empty string removes
// it. The list is shortened by the height of the block, so the sidebar keeps
// its size and nothing is hidden behind the footer.
func (m *Model) SetFooter(footer string) {
	m.footer = footer
}

// FooterAt reports whether the given component view coordinates point at the
// footer block.
func (m *Model) FooterAt(x, y int) bool {
	return m.footerRect.Contains(x, y)
}

// FooterRect is the area the footer occupied during the last render, in the
// coordinates of the component view.
func (m *Model) FooterRect() Rect {
	return m.footerRect
}

// SetFooterHover highlights the footer, so it looks like a button under the
// cursor.
func (m *Model) SetFooterHover(hover bool) {
	m.footerHover = hover
}
