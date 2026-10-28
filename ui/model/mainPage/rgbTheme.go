package mainpage

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/style"
)

// The rgb theme slowly cycles the hue of the accent color (and everything
// derived from it: borders, selection, lyrics dots). The animation reuses the
// usual discord tick, so there is no extra goroutine and no repaint storm: the
// hue advances once every 5 seconds, which reads as a smooth drift, not a
// strobe.

// rgbHueStep is how much the hue moves on every tick.
const rgbHueStep = 4

// rgbTickMsg carries the current hue around.
type rgbTickMsg float64

// rgbActive reports whether the animated theme is on.
func rgbActive() bool {
	return config.Current.Style != nil && config.Current.Style.Theme == "rgb"
}

// rgbStart returns the command that resumes the animation, if it should run.
func rgbStart() tea.Cmd {
	if !rgbActive() {
		return nil
	}
	return rgbTick()
}

func rgbTick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return rgbTickMsg(rgbNextHue(rgbHueStep))
	})
}

// rgbNextHue advances the stored hue and returns it.
func rgbNextHue(step float64) float64 {
	rgbHue += step
	if rgbHue >= 360 {
		rgbHue -= 360
	}
	return rgbHue
}

var rgbHue float64

// rgbApply re-colors the accent family with the current hue. Only the colors
// that follow the accent change; text and background stay put, so the screen
// never flashes.
func rgbApply() {
	if config.Current.Style == nil || config.Current.Style.Colors == nil {
		return
	}
	colors := *config.Current.Style.Colors
	accent := hslHex(rgbHue, 0.72, 0.60)
	colors.Accent = accent
	// the selection and the lyrics "previous" follow the accent, darker and
	// quieter, so the text stays readable
	colors.PlaylistSelection = hslHex(rgbHue, 0.30, 0.16)
	colors.LyricsPrevious = hslHex(rgbHue, 0.18, 0.25)
	config.Current.Style.Colors = &colors
	style.Apply(config.Current.Style)
}

// handleRgbTick advances the animation. The returned command continues it.
func (m *Model) handleRgbTick() tea.Cmd {
	if !rgbActive() {
		return nil
	}
	rgbApply()
	m.tracker.RefreshStyle()
	m.tracklist.RefreshStyle()
	m.playlists.RefreshStyle()
	return rgbTick()
}

// rgbStop resets the animation flag; the pending tick simply dies when it
// finds the theme switched off.

// hslHex renders an HSL color as a #rrggbb string.
func hslHex(h, s, l float64) string {
	r, g, b := hslToRgb(h, s, l)
	to := func(v float64) string {
		hex := "0123456789abcdef"
		c := int(v*255 + 0.5)
		if c > 255 {
			c = 255
		}
		if c < 0 {
			c = 0
		}
		return string([]byte{hex[c>>4], hex[c&15]})
	}
	return "#" + to(r) + to(g) + to(b)
}

// hslToRgb converts a hue in degrees with saturation and lightness in 0..1
// into RGB components in 0..1.
func hslToRgb(h, s, l float64) (r, g, b float64) {
	c := (1 - abs2(2*l-1)) * s
	hp := h / 60
	x := c * (1 - abs2(mod2(hp)-1))
	m := l - c/2

	switch {
	case hp < 1:
		r, g, b = c, x, 0
	case hp < 2:
		r, g, b = x, c, 0
	case hp < 3:
		r, g, b = 0, c, x
	case hp < 4:
		r, g, b = 0, x, c
	case hp < 5:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return r + m, g + m, b + m
}

// mod2 keeps only the fractional part of v modulo 2.
func mod2(v float64) float64 {
	return v - float64(int(v/2)*2)
}

func abs2(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
