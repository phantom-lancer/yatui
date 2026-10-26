package mainpage

import (
	"math"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/equalizer"
	"github.com/phantom-lancer/yatui/ui/style"
)

var eqAnsiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// eqStripANSI removes terminal escape codes from a string so the real cell
// positions can be measured.
func eqStripANSI(s string) string {
	return eqAnsiEscape.ReplaceAllString(s, "")
}

// _EQ_SETTINGS_WIDTH is the width of the equalizer window.
const _EQ_SETTINGS_WIDTH = 80

// _EQ_SLIDER_HEIGHT is the height of one vertical fader in cells.
const _EQ_SLIDER_HEIGHT = 11

// eqSlider is one vertical fader of the equalizer.
type eqSlider struct {
	band int
	x    int // screen x of the fader column centre
	y    int // screen y of the first fader row
	w    int // width of the fader column
	h    int // height of the fader column
}

// eqLayout is where the rendered dialog ended up on the screen, so the mouse
// can be mapped back to the faders and the toggle. All coordinates are
// measured from the actual rendered view, never hardcoded.
type eqLayout struct {
	x, y          int
	width, height int
	// toggleRect is the area of the on/off switch text.
	toggleRect Rect
	// presetRow is the screen row of the preset selector.
	presetRow int
	// applyRow is the screen row of the apply button.
	applyRow int
	// sliders are the faders, left to right.
	sliders []eqSlider
	valid   bool
}

// sliderAt returns the index of the fader under the given screen coordinates,
// or -1 when there is none.
func (l eqLayout) sliderAt(x, y int) int {
	for i, s := range l.sliders {
		if x >= s.x-s.w/2 && x < s.x+s.w/2 && y >= s.y && y < s.y+s.h {
			return i
		}
	}
	return -1
}

// eqModel is the window with the equalizer.
//
// index is the control under the cursor keys: -1 for the on/off toggle, 0
// for the preset selector and 1+band for a fader.
//
// dragging is the fader the mouse is moving, -1 when nothing is dragged.
type eqModel struct {
	enabled  bool
	preset   string
	gains    [equalizer.Bands]float64
	index    int
	dragging int
	layout   eqLayout
}

// openEqualizer shows the window with the equalizer.
func (m *Model) openEqualizer() tea.Cmd {
	enabled, gains := m.tracker.EQ()
	m.eq = &eqModel{
		enabled: enabled,
		preset:  config.Current.Equalizer.Preset,
		gains:   gains,
		index:   -1,
	}
	return tea.ClearScreen
}

// toggleEqualizer switches the equalizer on or off.
func (m *Model) toggleEqualizer() tea.Cmd {
	if m.eq == nil {
		m.eq = &eqModel{
			enabled: !config.Current.Equalizer.Enabled,
			preset:  config.Current.Equalizer.Preset,
			gains:   equalizer.NormalizeGains(config.Current.Equalizer.Gains),
			index:   -1,
		}
	} else {
		m.eq.enabled = !m.eq.enabled
		m.eq.index = -1
	}
	return nil
}

// presetList returns the names of the items the preset selector shows:
//
// "Своя настройка" means the gains were edited by hand, the rest are the
// ready made presets.
func presetList() []string {
	names := make([]string, 0, len(equalizer.Presets())+1)
	names = append(names, "Своя настройка")
	for _, preset := range equalizer.Presets() {
		names = append(names, preset.Name)
	}
	return names
}

// presetIndex returns the index of the preset with the given name in
// presetList.
func presetIndex(name string) int {
	names := presetList()
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return 0
}

// sliderValue converts a gain in decibels to the row of its fader handle.
func sliderValue(gain float64) int {
	row := int(math.Round((12 - gain) / 24 * float64(_EQ_SLIDER_HEIGHT-1)))
	if row < 0 {
		row = 0
	}
	if row > _EQ_SLIDER_HEIGHT-1 {
		row = _EQ_SLIDER_HEIGHT - 1
	}
	return row
}

// sliderGain converts the row of a fader handle back to a gain in decibels.
func sliderGain(row int) float64 {
	gain := 12 - float64(row)*24/float64(_EQ_SLIDER_HEIGHT-1)
	return equalizer.ClampGain(gain)
}

// sliderColumn renders one vertical fader.
//
// The column is 4 cells wide: the track is "│", the middle is "─", the
// handle is "●". When active the fader is painted in the accent color.
func sliderColumn(gain float64, active bool) string {
	knob := sliderValue(gain)

	lines := make([]string, _EQ_SLIDER_HEIGHT)
	for row := 0; row < _EQ_SLIDER_HEIGHT; row++ {
		var c string
		switch {
		case row == knob:
			c = "●"
		case row == _EQ_SLIDER_HEIGHT/2:
			c = "─"
		default:
			c = "│"
		}

		stl := lipgloss.NewStyle().Width(1).Align(lipgloss.Center)
		if active {
			stl = stl.Foreground(style.AccentColor)
		} else {
			stl = stl.Foreground(style.InactiveTextColor)
		}
		lines[row] = stl.Render(c)
	}

	// centre the single-cell column inside 4 cells
	out := make([]string, _EQ_SLIDER_HEIGHT)
	for row := 0; row < _EQ_SLIDER_HEIGHT; row++ {
		out[row] = padCenter(lines[row], 4)
	}
	return lipgloss.JoinVertical(lipgloss.Center, out...)
}

// padCenter centers a string in the given width.
func padCenter(s string, width int) string {
	if lipgloss.Width(s) >= width {
		return s
	}
	pad := width - lipgloss.Width(s)
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// labelWidthEQ is the width of the column with the names of the controls.
func labelWidthEQ() int {
	width := lipgloss.Width("Эквалайзер")
	if w := lipgloss.Width("Пресет"); w > width {
		width = w
	}
	return width
}

// eqView renders the window with the equalizer.
func (m *Model) eqView() string {
	title := style.DialogTitleStyle.Render("Эквалайзер")

	labelWidth := labelWidthEQ()

	// the toggle row: label on the left, the switch on the right
	toggle := "выкл"
	if m.eq.enabled {
		toggle = "вкл"
	}
	toggleStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(style.NormalTextColor)
	if m.eq.index == -1 {
		toggleStyle = toggleStyle.Foreground(style.InactiveTextColor).Background(style.AccentColor)
	}
	toggleRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Width(labelWidth).Render("Эквалайзер"),
		"  ",
		toggleStyle.Render(toggle),
	)

	// the preset row
	preset := m.eq.preset
	if preset == "" {
		preset = "Плоский"
	}
	presetStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(style.NormalTextColor)
	if m.eq.index == 0 {
		presetStyle = presetStyle.Foreground(style.InactiveTextColor).Background(style.AccentColor)
	}
	presetRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Width(labelWidth).Render("Пресет"),
		"  ",
		presetStyle.Render(preset),
	)

	// the faders: each column is 4 cells wide, separated by one space
	faderW := 4
	gap := 1
	total := labelWidth + 2 + equalizer.Bands*(faderW+gap) - gap
	if total > _EQ_SETTINGS_WIDTH-4 {
		gap = 0
		total = labelWidth + 2 + equalizer.Bands*(faderW+gap) - gap
	}

	// the level labels above the faders
	topLine := strings.Repeat(" ", labelWidth+2)
	for i := 0; i < equalizer.Bands; i++ {
		topLine += padCenter("+12", faderW) + strings.Repeat(" ", gap)
	}
	topLine = strings.TrimRight(topLine, " ")

	// the fader columns: every column is rendered to its own set of lines,
	// then the rows are joined side by side
	cols := make([][]string, equalizer.Bands)
	for i := 0; i < equalizer.Bands; i++ {
		cols[i] = strings.Split(sliderColumn(m.eq.gains[i], m.eq.index == 1+i), "\n")
	}
	midLines := make([]string, _EQ_SLIDER_HEIGHT)
	for row := 0; row < _EQ_SLIDER_HEIGHT; row++ {
		line := strings.Repeat(" ", labelWidth+2)
		for i := 0; i < equalizer.Bands; i++ {
			line += cols[i][row] + strings.Repeat(" ", gap)
		}
		midLines[row] = strings.TrimRight(line, " ")
	}

	// the frequency labels below the faders
	botLine := strings.Repeat(" ", labelWidth+2)
	for i := 0; i < equalizer.Bands; i++ {
		botLine += padCenter(equalizer.BandLabels[i], faderW) + strings.Repeat(" ", gap)
	}
	botLine = strings.TrimRight(botLine, " ")

	// the level labels at the bottom
	lvlLine := strings.Repeat(" ", labelWidth+2)
	for i := 0; i < equalizer.Bands; i++ {
		if i == 0 {
			lvlLine += padCenter("-12дБ", faderW)
		} else if i == equalizer.Bands/2 {
			lvlLine += padCenter("0дБ", faderW)
		} else if i == equalizer.Bands-1 {
			lvlLine += padCenter("+12дБ", faderW)
		} else {
			lvlLine += strings.Repeat(" ", faderW)
		}
		lvlLine += strings.Repeat(" ", gap)
	}
	lvlLine = strings.TrimRight(lvlLine, " ")

	faders := lipgloss.JoinVertical(lipgloss.Left,
		topLine,
		lipgloss.JoinVertical(lipgloss.Left, midLines...),
		botLine,
		lvlLine,
	)
	faderBlock := lipgloss.NewStyle().Padding(0, 1).Render(faders)

	rows := make([]string, 0, 4+equalizer.Bands)
	rows = append(rows, toggleRow)
	rows = append(rows, presetRow)
	rows = append(rows, faderBlock)
	rows = append(rows, "")

	// the apply row
	applyStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(style.NormalTextColor)
	if m.eq.index == 1+equalizer.Bands {
		applyStyle = applyStyle.Foreground(style.InactiveTextColor).Background(style.AccentColor)
	}
	rows = append(rows, applyStyle.Render("Применить"))

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)

	box := style.DialogBoxStyle.Width(_EQ_SETTINGS_WIDTH).Render(lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		body,
	))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		style.DialogHelpStyle.Render("↑/↓ — выбор, ←/→ — уровень, мышь — крутить ползунок, enter — применить, esc — выход"),
	)
}

// placeEqualizer draws the window in the middle of the screen and measures
// where every fader and the toggle landed in the actual rendered view, so
// the mouse can be mapped back correctly.
func (m *Model) placeEqualizer() string {
	view := m.eqView()
	placed := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, view)

	width := lipgloss.Width(view)
	height := lipgloss.Height(view)
	x := (m.width - width) / 2
	y := (m.height - height) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	lines := strings.Split(view, "\n")

	// find the rows of the controls by scanning the rendered lines
	toggleY := -1
	toggleX := -1
	presetY := -1
	applyY := -1
	for i, line := range lines {
		strip := eqStripANSI(line)
		switch {
		case toggleY < 0 && (strings.Contains(strip, "вкл") || strings.Contains(strip, "выкл")):
			toggleY = y + i
			// the x of "вкл"/"выкл" inside the stripped line
			idx := strings.IndexAny(strip, "вв") // first cyrillic v of вкл/выкл
			if idx < 0 {
				idx = strings.Index(strip, "вкл")
				if idx < 0 {
					idx = strings.Index(strip, "выкл")
				}
			}
			if idx >= 0 {
				toggleX = x + idx
			}
		case presetY < 0 && strings.Contains(strip, "Пресет"):
			presetY = y + i
		case applyY < 0 && strings.Contains(strip, "Применить"):
			applyY = y + i
		}
	}

	// measure the fader columns from the frequency labels row: each label is
	// centred over its fader, so its x position is the fader centre.
	sliders := make([]eqSlider, 0, equalizer.Bands)
	for i := 0; i < equalizer.Bands; i++ {
		// scan every line for the frequency label of band i
		for _, line := range lines {
			strip := eqStripANSI(line)
			idx := strings.Index(strip, equalizer.BandLabels[i])
			if idx >= 0 {
				sliders = append(sliders, eqSlider{
					band: i,
					x:    x + idx + lipgloss.Width(equalizer.BandLabels[i])/2,
					y:    y + 6, // faders start 6 rows below the box top
					w:    4,
					h:    _EQ_SLIDER_HEIGHT,
				})
				break
			}
		}
	}

	m.eq.layout = eqLayout{
		x:          x,
		y:          y,
		width:      width,
		height:     height,
		toggleRect: Rect{X: toggleX, Y: toggleY, Width: 4, Height: 1},
		sliders:    sliders,
		presetRow:  presetY,
		applyRow:   applyY,
		valid:      toggleY >= 0 && presetY >= 0 && applyY >= 0,
	}

	return placed
}

// eqControl handles the keys and the mouse while the window is open.
func (m *Model) eqControl(message tea.Msg) (tea.Cmd, bool) {
	if m.eq == nil {
		return nil, false
	}

	switch msg := message.(type) {
	case tea.KeyMsg:
		keypress := msg.String()

		switch keypress {
		case "esc":
			m.eq = nil
			return tea.ClearScreen, true
		case "up":
			m.eq.index--
			if m.eq.index < -1 {
				m.eq.index = 1 + equalizer.Bands
			}
			return nil, true
		case "down":
			m.eq.index++
			if m.eq.index > 1+equalizer.Bands {
				m.eq.index = -1
			}
			return nil, true
		case "left", "right":
			return m.eqChange(keypress == "right"), true
		case "enter", " ":
			if m.eq.index == -1 {
				m.eq.enabled = !m.eq.enabled
				return nil, true
			}
			return m.eqApply(), true
		}

		// the window swallows every other key
		return nil, true

	case tea.MouseMsg:
		return m.eqMouse(msg), true
	}

	return nil, false
}

// eqChange changes the value of the selected control: dir is 1 for "right",
// -1 for "left".
func (m *Model) eqChange(right bool) tea.Cmd {
	dir := 1
	if !right {
		dir = -1
	}

	switch m.eq.index {
	case -1:
		// the toggle
		m.eq.enabled = !m.eq.enabled
		return nil

	case 0:
		// the preset
		presets := presetList()
		index := presetIndex(m.eq.preset)
		index = (index + dir + len(presets)) % len(presets)
		name := presets[index]
		m.eq.preset = name
		if name == "Своя настройка" {
			m.eq.enabled = true
			return nil
		}
		m.eq.enabled = true
		if preset, ok := equalizer.PresetByName(name); ok {
			m.eq.gains = preset.Gains
		}
		return nil

	default:
		// a fader
		band := m.eq.index - 1
		if band < 0 || band >= equalizer.Bands {
			return nil
		}
		m.eq.enabled = true
		m.eq.preset = "Своя настройка"
		gain := m.eq.gains[band] + float64(dir)
		m.eq.gains[band] = equalizer.ClampGain(gain)
		m.tracker.SetEQ(true, m.eq.gains)
		return nil
	}
}

// eqApply applies the settings of the window and closes it.
func (m *Model) eqApply() tea.Cmd {
	if m.eq == nil {
		return nil
	}

	enabled := m.eq.enabled
	config.Current.Equalizer.Enabled = enabled
	config.Current.Equalizer.Preset = m.eq.preset
	config.Current.Equalizer.Gains = m.eq.gains[:]
	m.saveConfig()

	m.tracker.SetEQ(enabled, m.eq.gains)

	m.eq = nil
	m.showToast("Эквалайзер: " + onOff(enabled))
	return tea.ClearScreen
}

// eqMouse handles the mouse while the window is open.
func (m *Model) eqMouse(msg tea.MouseMsg) tea.Cmd {
	if m.eq == nil || !m.eq.layout.valid {
		return nil
	}
	layout := m.eq.layout

	switch msg.Action {
	case tea.MouseActionMotion:
		if msg.Button != tea.MouseButtonNone {
			// dragging a fader
			if m.eq.dragging >= 0 && m.eq.dragging < equalizer.Bands {
				s := layout.sliders[m.eq.dragging]
				// map the y position inside the fader to a gain
				row := msg.Y - s.y
				if row < 0 {
					row = 0
				}
				if row > _EQ_SLIDER_HEIGHT-1 {
					row = _EQ_SLIDER_HEIGHT - 1
				}
				m.eq.enabled = true
				m.eq.preset = "Своя настройка"
				m.eq.gains[m.eq.dragging] = sliderGain(row)
				m.tracker.SetEQ(true, m.eq.gains)
				return nil
			}
			break
		}
		// hover: highlight the control under the cursor
		if idx := layout.sliderAt(msg.X, msg.Y); idx >= 0 {
			if m.eq.index != 1+idx {
				m.eq.index = 1 + idx
			}
		} else if layout.toggleRect.Contains(msg.X, msg.Y) {
			if m.eq.index != -1 {
				m.eq.index = -1
			}
		}

	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonMiddle {
			break
		}

		// the toggle
		if layout.toggleRect.Contains(msg.X, msg.Y) {
			m.eq.enabled = !m.eq.enabled
			m.eq.index = -1
			m.tracker.SetEQ(m.eq.enabled, m.eq.gains)
			return nil
		}

		// a fader: click sets the level, press-and-hold drags it
		if idx := layout.sliderAt(msg.X, msg.Y); idx >= 0 {
			m.eq.dragging = idx
			m.eq.index = 1 + idx
			s := layout.sliders[idx]
			row := msg.Y - s.y
			if row < 0 {
				row = 0
			}
			if row > _EQ_SLIDER_HEIGHT-1 {
				row = _EQ_SLIDER_HEIGHT - 1
			}
			m.eq.enabled = true
			m.eq.preset = "Своя настройка"
			m.eq.gains[idx] = sliderGain(row)
			m.tracker.SetEQ(true, m.eq.gains)
			return nil
		}

		// the preset row
		if msg.Y == layout.presetRow {
			m.eq.index = 0
			return m.eqChange(msg.X > layout.x+layout.width/2)
		}

		// the apply row
		if msg.Y == layout.applyRow {
			return m.eqApply()
		}

		// a click outside of the window closes it
		if msg.X < layout.x || msg.X >= layout.x+layout.width || msg.Y < layout.y || msg.Y >= layout.y+layout.height {
			m.eq = nil
			return tea.ClearScreen
		}

	case tea.MouseActionRelease:
		m.eq.dragging = -1
	}

	return nil
}
