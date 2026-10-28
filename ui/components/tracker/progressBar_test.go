package tracker

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// TestRenderProgressBarWidth makes sure the bar always takes exactly the
// requested number of cells, whatever the fraction.
func TestRenderProgressBarWidth(t *testing.T) {
	full := lipgloss.Color("#FC0")
	empty := lipgloss.Color("#6b6b6b")

	for _, percent := range []float64{0, 0.001, 0.33, 0.5, 0.999, 1, -1, 2} {
		for _, width := range []int{1, 7, 40, 80} {
			bar := renderProgressBar(percent, width, full, empty)
			if got := ansi.StringWidth(bar); got != width {
				t.Errorf("renderProgressBar(%.3f, %d) is %d cells, want %d", percent, width, got, width)
			}
		}
	}
}

// TestRenderProgressBarSubCell checks that the bar uses partial blocks between
// full cells, which is the whole point: whole cells alone make it jump.
func TestRenderProgressBarSubCell(t *testing.T) {
	full := lipgloss.Color("#FC0")
	empty := lipgloss.Color("#6b6b6b")

	// a quarter of 40 cells is 10 cells, an eighth more is a partial block
	bar := ansi.Strip(renderProgressBar(0.25625, 40, full, empty))
	partial := false
	for _, r := range partialBlocks {
		if strings.ContainsRune(bar, r) {
			partial = true
			break
		}
	}
	if !partial {
		t.Fatalf("the bar has no partial block: %q", bar)
	}

	// no partial block when the position lands on a whole cell
	bar = ansi.Strip(renderProgressBar(0.5, 40, full, empty))
	for _, r := range partialBlocks {
		if strings.ContainsRune(bar, r) {
			t.Fatalf("a whole cell position must not use a partial block: %q", bar)
		}
	}
}

// TestRenderProgressBarEdges checks the two ends of the track.
func TestRenderProgressBarEdges(t *testing.T) {
	full := lipgloss.Color("#FC0")
	empty := lipgloss.Color("#6b6b6b")

	if got := ansi.Strip(renderProgressBar(0, 4, full, empty)); got != "████" {
		t.Errorf("empty bar is %q", got)
	}
	if got := ansi.Strip(renderProgressBar(1, 4, full, empty)); got != "████" {
		t.Errorf("full bar is %q", got)
	}
	// the width of the rendered bar must not depend on the fraction
	if a, b := ansi.StringWidth(renderProgressBar(0, 9, full, empty)), ansi.StringWidth(renderProgressBar(1, 9, full, empty)); a != b {
		t.Errorf("bar width depends on the fraction: %d vs %d", a, b)
	}
}