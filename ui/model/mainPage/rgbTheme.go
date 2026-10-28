package mainpage

import (
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/style"
)

// The rgb theme slowly cycles the hue of the accent color and everything
// derived from it. It runs on its own cheap timer: one small color update per
// tick, no 60 fps loop, so the CPU cost stays near zero.

const (
	// rgbTickInterval is how often the hue advances.
	rgbTickInterval = 200 * time.Millisecond
	// rgbDefaultSpeed is the middle of the speed slider.
	rgbDefaultSpeed = 5
	// rgbMaxSpeed is the fastest step of the slider.
	rgbMaxSpeed = 10
)

// rgbTickMsg carries the current hue around.
type rgbTickMsg float64

// rgbActive reports whether the animated theme is on.
func rgbActive() bool {
	return config.Current.Style != nil && config.Current.Style.Theme == "rgb"
}

// rgbSpeed returns the configured speed in 1..rgbMaxSpeed ticks per step.
func rgbSpeed() int {
	if config.Current.RgbSpeed < 1 {
		return rgbDefaultSpeed
	}
	if config.Current.RgbSpeed > rgbMaxSpeed {
		return rgbMaxSpeed
	}
	return config.Current.RgbSpeed
}

// rgbStart returns the command that starts the animation, if it should run.
// It is safe to call repeatedly: only one loop can be running.
func (m *Model) rgbStart() tea.Cmd {
	if !rgbActive() {
		m.rgbRunning = false
		return nil
	}
	if m.rgbRunning {
		return nil
	}
	m.rgbRunning = true
	return rgbTick()
}

func rgbTick() tea.Cmd {
	return tea.Tick(rgbTickInterval, func(time.Time) tea.Msg {
		return rgbTickMsg(rgbHue)
	})
}

var rgbHue float64

// rgbAdvance moves the hue by one step. The step grows with the speed slider,
// so "fast" really is faster: at the default 5 the accent makes a full circle
// in about 18 seconds.
func rgbAdvance() {
	rgbHue += float64(rgbSpeed()) * 0.8
	for rgbHue >= 360 {
		rgbHue -= 360
	}
}

// rgbApply re-colors the accent family with the current hue. Only the colors
// that follow the accent change; text and background stay put, so the screen
// never flashes.
func rgbApply() {
	if config.Current.Style == nil || config.Current.Style.Colors == nil {
		return
	}
	colors := *config.Current.Style.Colors
	colors.Accent = hslHex(rgbHue, 0.72, 0.62)
	// the selection and the lyrics "previous" follow the accent, darker and
	// quieter, so the text stays readable
	colors.PlaylistSelection = hslHex(rgbHue, 0.30, 0.16)
	colors.LyricsPrevious = hslHex(rgbHue, 0.18, 0.25)
	config.Current.Style.Colors = &colors
	style.Apply(config.Current.Style)
}

// handleRgbTick advances the animation. The returned command continues it, or
// is nil once the theme is switched off.
func (m *Model) handleRgbTick() tea.Cmd {
	if !rgbActive() {
		m.rgbRunning = false
		return nil
	}
	rgbAdvance()
	rgbApply()
	m.tracker.RefreshStyle()
	m.tracklist.RefreshStyle()
	m.playlists.RefreshStyle()
	return rgbTick()
}

// applyRgbSpeed saves a new speed, restarts the animation with it and shows
// the choice.
func (m *Model) applyRgbSpeed(speed int) tea.Cmd {
	if speed < 1 {
		speed = 1
	}
	if speed > rgbMaxSpeed {
		speed = rgbMaxSpeed
	}
	config.Current.RgbSpeed = speed
	m.saveConfig()
	m.showToast("Скорость RGB: " + strconv.Itoa(speed))
	return tea.ClearScreen
}

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

// mod2 keeps the fractional part of v modulo 2.
func mod2(v float64) float64 {
	return v - float64(int(v/2)*2)
}

func abs2(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
