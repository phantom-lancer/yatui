package mainpage

import (
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/equalizer"
	"github.com/phantom-lancer/yatui/ui/style"
)

// _EQ_SETTINGS_WIDTH is the width of the equalizer window.
const _EQ_SETTINGS_WIDTH = 64

// _EQ_SLIDER_HEIGHT is the height of one vertical fader in cells.
const _EQ_SLIDER_HEIGHT = 11

// eqSlider is one vertical fader of the equalizer.
type eqSlider struct {
	band int
	x    int
	y    int
	w    int
	h    int
}

// eqLayout is where the rendered dialog ended up on the screen, so the mouse
// can be mapped back to the faders and the toggle. All coordinates are
// computed directly while rendering, never parsed back from the view.
type eqLayout struct {
	x, y          int
	width, height int
	// toggleRect is the area of the on/off switch on the first row.
	toggleRect Rect
	// sliders are the faders, left to right.
	sliders []eqSlider
	// presetRow is the screen row of the preset selector.
	presetRow int
	// applyRow is the screen row of the apply button.
	applyRow int
	valid    bool
}

// sliderAt returns the index of the fader under the given screen coordinates,
// or -1 when there is none.
func (l eqLayout) sliderAt(x, y int) int {
	for i, s := range l.sliders {
		if x >= s.x && x < s.x+s.w && y >= s.y && y < s.y+s.h {
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
// The column is one cell wide: the track is "│", the middle is "─", the
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

	return lipgloss.JoinVertical(lipgloss.Center, lines...)
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

	// the faders: each column is one cell wide, separated by one space
	faderW := 5
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

	box := style.DialogBoxStyle.Render(lipgloss.JoinVertical(
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

// placeEqualizer draws the window in the middle of the screen and remembers
// where every fader and the toggle landed, so the mouse can be mapped back.
// The geometry is computed directly: the view is never parsed.
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

	box := style.DialogBoxStyle
	labelW := labelWidthEQ()

	// row layout inside the box:
	// 0: title (with margin bottom)  -> 2 lines
	// 2: toggle row
	// 3: preset row
	// 4: blank
	// 5: top level labels
	// 6..16: faders (11 rows)
	// 17: frequency labels
	// 18: level labels
	// 19: blank
	// 20: apply row
	toggleY := y + box.GetBorderTopSize() + box.GetPaddingTop() + 2
	presetY := toggleY + 1
	sliderY := y + box.GetBorderTopSize() + box.GetPaddingTop() + 6

	toggleX := x + box.GetBorderLeftSize() + box.GetPaddingLeft() + labelW + 2

	sliderX := toggleX
	sliderW := 1
	sliders := make([]eqSlider, 0, equalizer.Bands)
	for i := 0; i < equalizer.Bands; i++ {
		sliders = append(sliders, eqSlider{
			band: i,
			x:    sliderX,
			y:    sliderY,
			w:    sliderW,
			h:    _EQ_SLIDER_HEIGHT,
		})
		sliderX += sliderW + 1
	}

	applyY := y + box.GetBorderTopSize() + box.GetPaddingTop() + 20

	m.eq.layout = eqLayout{
		x:          x,
		y:          y,
		width:      width,
		height:     height,
		toggleRect: Rect{X: toggleX, Y: toggleY, Width: 6, Height: 1},
		sliders:    sliders,
		presetRow:  presetY,
		applyRow:   applyY,
		valid:      true,
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
	if m.eq == nil {
		return nil
	}
	layout := m.eq.layout

	switch msg.Action {
	case tea.MouseActionMotion:
		if msg.Button != tea.MouseButtonNone {
			// dragging a fader
			if m.eq.dragging >= 0 && m.eq.dragging < equalizer.Bands {
				s := layout.sliders[m.eq.dragging]
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
