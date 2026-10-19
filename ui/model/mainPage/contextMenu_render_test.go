package mainpage

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// colored builds a line with a colored middle part, like the real UI does.
func colored(prefix, mid, suffix string) string {
	return prefix + "\x1b[38;2;68;68;68m" + mid + "\x1b[0m" + suffix
}

func TestOverlayKeepsAnsiIntact(t *testing.T) {
	// base line: "AAAA[colored BBBB]CCCC"
	base := colored("AAAA", "BBBB", "CCCC")
	menu := "PLAY"

	got := overlay(base, menu, 0)

	// no ANSI fragment may leak into the visible text
	if visible := stripANSI(got); strings.Contains(visible, "m") && strings.Contains(visible, "8;") {
		t.Errorf("ANSI fragment leaked into the output: %q", visible)
	}

	// the base content after the overlay must survive
	if !strings.Contains(stripANSI(got), "CCCC") {
		t.Errorf("right part of the base line is lost: %q", stripANSI(got))
	}

	// every line must have the same cell width as the original
	if w, bw := ansi.StringWidth(got), ansi.StringWidth(base); w != bw {
		t.Errorf("line width changed: got %d, want %d (%q)", w, bw, stripANSI(got))
	}
}

func TestOverlayKeepsLineWidth(t *testing.T) {
	base := colored("title", "artist", "2:41")
	for _, x := range []int{0, 3, 6, 10} {
		got := overlay(base, "MENU", x)
		if w := ansi.StringWidth(got); w < ansi.StringWidth(base) {
			t.Errorf("x=%d: line got shorter: %d < %d", x, w, ansi.StringWidth(base))
		}
	}
}

func TestOverlayPadsShortLine(t *testing.T) {
	// a line that is shorter than the menu plus its offset
	base := "short"
	got := overlay(base, "MENU", 20)
	if ansi.StringWidth(got) < 24 {
		t.Errorf("line was not padded: %d", ansi.StringWidth(got))
	}
	if !strings.Contains(stripANSI(got), "short") {
		t.Errorf("base content is lost: %q", stripANSI(got))
	}
}

func TestRenderDoesNotBreakColoredBase(t *testing.T) {
	// A base view resembling the real one: colored rows, wide enough
	// to host the menu without being resized.
	// Colors are applied to whole ASCII segments so that multibyte
	// box drawing characters are never split by an escape sequence.
	const rowWidth = 63

	// a row of exactly rowWidth cells with a colored title segment
	coloredRow := func() string {
		left := "| "
		title := "first track"
		rest := "| artist 2:41 | more text here  "
		if ansi.StringWidth(left)+ansi.StringWidth(title)+ansi.StringWidth(rest) > rowWidth {
			t.Fatalf("the row is wider than %d cells", rowWidth)
		}
		pad := rowWidth - ansi.StringWidth(left) - ansi.StringWidth(title) - ansi.StringWidth(rest)

		return left +
			"\x1b[38;2;255;204;0m" + title + "\x1b[0m" +
			rest + strings.Repeat(" ", pad)
	}

	base := strings.Join([]string{
		"+" + strings.Repeat("-", rowWidth-2) + "+",
		coloredRow(),
		coloredRow(),
		coloredRow(),
		"+" + strings.Repeat("-", 40) + "+" + strings.Repeat("-", rowWidth-43) + "+",
		coloredRow(),
		coloredRow(),
		"+" + strings.Repeat("-", rowWidth-2) + "+",
	}, "\n")

	baseLines := strings.Split(base, "\n")
	baseWidth := ansi.StringWidth(baseLines[0])

	cm := &contextMenu{
		x:     4,
		y:     1,
		items: []contextMenuItem{{label: "Play"}, {label: "Share"}},
	}
	w, h := cm.size()
	if 4+w > baseWidth {
		t.Fatalf("the test base is too narrow for the menu: %d < %d", baseWidth, 4+w)
	}

	got := cm.render(base)
	gotLines := strings.Split(got, "\n")
	if len(gotLines) != len(baseLines) {
		t.Fatalf("render changed the line count: %d", len(gotLines))
	}

	// every line must keep its exact cell width
	for i, line := range gotLines {
		if w := ansi.StringWidth(line); w != baseWidth {
			t.Errorf("line %d width changed: got %d want %d\n  %q", i, w, baseWidth, stripANSI(line))
		}
	}

	// no escape sequence may leak into the visible text
	for i, line := range gotLines {
		if visible := stripANSI(line); strings.ContainsRune(visible, 0x1b) {
			t.Errorf("line %d has a leaked escape sequence: %q", i, visible)
		}
	}

	// the menu must be rendered
	visible := stripANSI(got)
	if !strings.Contains(visible, "Play") || !strings.Contains(visible, "Share") {
		t.Errorf("menu items are not rendered:\n%s", visible)
	}

	// the rows below the menu must stay intact
	for i := cm.y + h; i < len(gotLines); i++ {
		if stripANSI(gotLines[i]) != stripANSI(baseLines[i]) {
			t.Errorf("line %d below the menu was damaged:\n  got  %q\n  want %q",
				i, stripANSI(gotLines[i]), stripANSI(baseLines[i]))
		}
	}
}

func stripANSI(s string) string {
	return ansi.Strip(s)
}
