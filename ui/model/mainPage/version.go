package mainpage

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/dece2183/yamusic-tui/ui/style"
)

// Version is shown in the top right corner of the player.
const Version = "v0.6.5"

// drawVersion puts the version label into the top right corner of the view.
// It does not change the view itself: the label simply replaces the cells
// under it, so the layout stays exactly the same.
func drawVersion(view string) string {
	lines := strings.Split(view, "\n")
	if len(lines) == 0 {
		return view
	}

	label := " " + Version + " "

	width := ansi.StringWidth(lines[0])
	if width < ansi.StringWidth(label)+1 {
		return view
	}

	// the label is painted on the first line, in the right corner
	x := width - ansi.StringWidth(label)
	lines[0] = overlay(lines[0], style.AccentTextStyle.Render(label), x)

	return strings.Join(lines, "\n")
}
