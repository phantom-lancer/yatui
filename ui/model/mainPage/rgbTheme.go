package mainpage

import (
	"math"
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
	// rgbDefaultFrameMs is the frame of the animation when the config says
	// nothing: five repaints a second.
	rgbDefaultFrameMs = 200
	// rgbMinFrameMs and rgbMaxFrameMs limit the smoothness setting. Every
	// frame repaints the whole window, so the frame is the main cost of the
	// animated theme: the fastest one is twelve times heavier than the
	// cheapest.
	rgbMinFrameMs = 40
	rgbMaxFrameMs = 500
	// rgbDefaultSpeed is the middle of the speed slider.
	rgbDefaultSpeed = 5
	// rgbMaxSpeed is the fastest step of the slider.
	rgbMaxSpeed = 10
)

// rgbTickMsg carries the current hue around.
type rgbTickMsg float64

// rgbFrameMs is the configured frame of the animation, in milliseconds.
func rgbFrameMs() int {
	ms := config.Current.RgbFrameMs
	switch {
	case ms <= 0:
		return rgbDefaultFrameMs
	case ms < rgbMinFrameMs:
		return rgbMinFrameMs
	case ms > rgbMaxFrameMs:
		return rgbMaxFrameMs
	}
	return ms
}

// rgbFrameInterval is the frame of the animation as a duration.
func rgbFrameInterval() time.Duration {
	return time.Duration(rgbFrameMs()) * time.Millisecond
}

// rgbActive reports whether the animated theme is on.
func rgbActive() bool {
	return !config.Current.ResourceSaver &&
		config.Current.Style != nil && config.Current.Style.Theme == "rgb"
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
	return tea.Tick(rgbFrameInterval(), func(time.Time) tea.Msg {
		return rgbTickMsg(rgbHue)
	})
}

var rgbHue float64

// rgbAdvance moves the hue by one step. The step grows with the speed slider,
// so "fast" really is faster: at the default 5 the accent makes a full circle
// in about 18 seconds. The step follows the frame, so a smoother or a cheaper
// repaint does not change the speed of the animation.
func rgbAdvance() {
	rgbHue += float64(rgbSpeed()) * 4 * rgbFrameInterval().Seconds()
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

// applyRgbFrame saves a new smoothness of the animation. The next tick is
// scheduled with the new frame, so the change is felt at once.
func (m *Model) applyRgbFrame(ms int) tea.Cmd {
	if ms < rgbMinFrameMs {
		ms = rgbMinFrameMs
	} else if ms > rgbMaxFrameMs {
		ms = rgbMaxFrameMs
	}
	config.Current.RgbFrameMs = ms
	m.saveConfig()
	m.showToast("Плавность RGB: " + rgbFrameLabel(ms))
	return tea.ClearScreen
}

// rgbFrameLabel describes a frame, in milliseconds, the way the user sees it:
// the name of the smoothness and the number of frames the screen gets.
func rgbFrameLabel(ms int) string {
	fps := 1000 / float64(ms)

	name := "безумно"
	switch {
	case fps < 3:
		name = "экономно"
	case fps < 4.5:
		name = "спокойно"
	case fps < 6:
		name = "обычно"
	case fps < 9:
		name = "плавно"
	case fps < 12:
		name = "очень плавно"
	case fps < 20:
		name = "максимум"
	}

	rounded := math.Round(fps*10) / 10
	return name + " · " + strconv.FormatFloat(rounded, 'f', -1, 64) + " к/с"
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
