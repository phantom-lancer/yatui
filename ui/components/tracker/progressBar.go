package tracker

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The progress bar used to be redrawn only in whole cells: on a four minute
// track a 80 cell bar moves one cell every ~3 seconds, which reads as a stutter
// no spring can hide. These eighth blocks give the bar sub-cell resolution, so
// it advances a fraction of a cell on every update instead.
var partialBlocks = [7]rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉'}

// renderProgressBar draws the bar with eighth-of-a-cell precision. The partial
// cell is a block glyph with the empty color as its background, so the unfilled
// part of that cell matches the empty cells exactly.
func renderProgressBar(percent float64, width int, full, empty lipgloss.Color) string {
	if width <= 0 {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}

	filled := percent * float64(width)
	whole := int(filled)
	eighths := int((filled-float64(whole))*8 + 0.5)
	if eighths >= 8 {
		whole++
		eighths = 0
	}
	if whole > width {
		whole = width
		eighths = 0
	}

	fullCell := lipgloss.NewStyle().Foreground(full).Render("█")
	emptyCell := lipgloss.NewStyle().Foreground(empty).Render("█")

	var b strings.Builder
	b.Grow(width * 12)
	b.WriteString(strings.Repeat(fullCell, whole))

	rest := width - whole
	if eighths > 0 && rest > 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(full).
			Background(empty).
			Render(string(partialBlocks[eighths-1])))
		rest--
	}

	b.WriteString(strings.Repeat(emptyCell, rest))
	return b.String()
}
