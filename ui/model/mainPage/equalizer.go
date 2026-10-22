package mainpage

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/equalizer"
	"github.com/phantom-lancer/yatui/ui/style"
)

// _EQ_SETTINGS_WIDTH is the width of the equalizer window.
const _EQ_SETTINGS_WIDTH = 44

// eqModel is the window with the equalizer.
type eqModel struct {
	enabled bool
	preset  string
	gains   [equalizer.Bands]float64
	// index is the row under the cursor.
	index int
	// layout is where the rows of the window ended up on the screen.
	layout eqLayout
}

// eqLayout is where the rendered dialog ended up on the screen, so the mouse
// can be mapped back to the rows.
type eqLayout struct {
	x, y          int
	width, height int
	rowX, rowY    int
	rowWidth      int
	labelWidth    int
	rows          int
	valid         bool
}

// rowAt returns the index of the row under the given screen coordinates.
func (l eqLayout) rowAt(x, y int) (int, bool) {
	if !l.valid || x < l.x || x >= l.x+l.width || y < l.rowY {
		return 0, false
	}
	if row := y - l.rowY; row < l.rows {
		return row, true
	}
	return 0, false
}

// valueDir reports which part of the value the cursor is on: -1 for the left
// half, 1 for the right one and 0 for the rest of the row.
func (l eqLayout) valueDir(x int) int {
	valueX := l.rowX + l.labelWidth + 2
	valueWidth := l.rowWidth - l.labelWidth - 2
	if x < valueX || valueWidth < 2 {
		return 0
	}
	if x >= valueX+valueWidth {
		return 0
	}
	if 2*(x-valueX) < valueWidth {
		return -1
	}
	return 1
}

// presetList returns the names of the items the preset selector shows: "Выкл"
// turns the equalizer off, the rest are the ready made presets.
func presetList() []string {
	names := make([]string, 0, len(equalizer.Presets())+1)
	names = append(names, "Выкл")
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

// openEqualizer shows the window with the equalizer.
func (m *Model) openEqualizer() tea.Cmd {
	enabled, gains := m.tracker.EQ()
	m.eq = &eqModel{
		enabled: enabled,
		preset:  config.Current.Equalizer.Preset,
		gains:   gains,
		index:   0,
	}
	return tea.ClearScreen
}

// toggleEqualizer switches the equalizer on or off.
func (m *Model) toggleEqualizer() tea.Cmd {
	m.eq.enabled = !m.eq.enabled
	return nil
}

// eqView renders the window with the equalizer.
func (m *Model) eqView() string {
	title := style.DialogTitleStyle.Render("Эквалайзер")

	labelWidth := labelWidthEQ()
	valueWidth := _EQ_SETTINGS_WIDTH - labelWidth - 2
	if valueWidth < 8 {
		valueWidth = 8
	}

	rows := make([]string, 0, 2+equalizer.Bands+2)

	// the switch
	switchValue := "выкл"
	if m.eq.enabled {
		switchValue = "вкл"
	}
	switchCell := padRight(switchValue, valueWidth)
	if m.eq.index == 0 && valueWidth >= 6 {
		switchCell = "‹ " + padRight(switchValue, valueWidth-4) + " ›"
	}
	line := padRight("Эквалайзер", labelWidth) + "  " + switchCell
	rows = append(rows, eqRowStyle(line, m.eq.index == 0))

	// the preset
	preset := m.eq.preset
	if preset == "" {
		preset = "Плоский"
	}
	presetCell := padRight(preset, valueWidth)
	if m.eq.index == 1 && valueWidth >= 6 {
		presetCell = "‹ " + padRight(preset, valueWidth-4) + " ›"
	}
	line = padRight("Пресет", labelWidth) + "  " + presetCell
	rows = append(rows, eqRowStyle(line, m.eq.index == 1))

	// the bands
	for i := 0; i < equalizer.Bands; i++ {
		gain := m.eq.gains[i]
		valueCell := padRight(formatGain(gain), valueWidth)
		if m.eq.index == 2+i && valueWidth >= 6 {
			valueCell = "‹ " + padRight(formatGain(gain), valueWidth-4) + " ›"
		}
		line = padRight(equalizer.BandLabels[i], labelWidth) + "  " + valueCell
		rows = append(rows, eqRowStyle(line, m.eq.index == 2+i))
	}

	rows = append(rows, "")

	// the reset and the apply rows
	resetCell := "Сбросить"
	if m.eq.index == 2+equalizer.Bands && valueWidth >= 6 {
		resetCell = "‹ " + padRight(resetCell, valueWidth-4) + " ›"
	}
	rows = append(rows, eqRowStyle(padRight("Сбросить", labelWidth)+"  "+resetCell, m.eq.index == 2+equalizer.Bands))

	applyCell := "Применить"
	if m.eq.index == 2+equalizer.Bands+1 && valueWidth >= 6 {
		applyCell = "‹ " + padRight(applyCell, valueWidth-4) + " ›"
	}
	rows = append(rows, eqRowStyle(padRight("Применить", labelWidth)+"  "+applyCell, m.eq.index == 2+equalizer.Bands+1))

	box := style.DialogBoxStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		style.DialogHelpStyle.Render("↑/↓ — выбор, ←/→ — изменить, enter — применить, esc — выход"),
	)
}

// eqRowStyle formats one row: highlighted when it is the one under the cursor.
func eqRowStyle(line string, active bool) string {
	rowStyle := lipgloss.NewStyle().Padding(0, 1)
	if active {
		rowStyle = rowStyle.
			Foreground(style.InactiveTextColor).
			Background(style.AccentColor)
	} else {
		rowStyle = rowStyle.Foreground(style.NormalTextColor)
	}
	return rowStyle.Render(line)
}

// labelWidthEQ is the width of the column with the names of the controls.
func labelWidthEQ() int {
	width := lipgloss.Width("Эквалайзер")
	if w := lipgloss.Width("Пресет"); w > width {
		width = w
	}
	for _, label := range equalizer.BandLabels {
		if w := lipgloss.Width(label); w > width {
			width = w
		}
	}
	return width
}

// formatGain formats a gain value the way it is shown in the window.
func formatGain(gain float64) string {
	if gain == 0 {
		return "±0"
	}
	sign := "+"
	if gain < 0 {
		sign = "-"
		gain = -gain
	}
	return sign + strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", gain), "0"), ".")
}

// placeEqualizer draws the window in the middle of the screen and remembers
// where every row landed.
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
	m.eq.layout = eqLayout{
		x:          x,
		y:          y,
		width:      width,
		height:     height,
		rowX:       x + box.GetBorderLeftSize() + box.GetPaddingLeft() + 1,
		rowY:       y + box.GetBorderTopSize() + box.GetPaddingTop() + 2,
		rowWidth:   _EQ_SETTINGS_WIDTH,
		labelWidth: labelWidthEQ(),
		rows:       2 + equalizer.Bands + 2,
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
			if m.eq.index < 0 {
				m.eq.index = m.eq.layout.rows - 1
			}
			return nil, true
		case "down":
			m.eq.index++
			if m.eq.index >= m.eq.layout.rows {
				m.eq.index = 0
			}
			return nil, true
		case "left", "right":
			return m.eqChange(keypress == "right"), true
		case "enter", " ":
			return m.eqApply(), true
		}

		// the window swallows every other key
		return nil, true

	case tea.MouseMsg:
		return m.eqMouse(msg), true
	}

	return nil, false
}

// eqChange changes the value of the selected row: dir is 1 for "right",
// -1 for "left".
func (m *Model) eqChange(right bool) tea.Cmd {
	dir := 1
	if !right {
		dir = -1
	}

	switch m.eq.index {
	case 0:
		// the switch
		m.eq.enabled = !m.eq.enabled
		return nil

	case 1:
		// the preset
		presets := presetList()
		index := presetIndex(m.eq.preset)
		index = (index + dir + len(presets)) % len(presets)
		name := presets[index]
		if name == "Выкл" {
			m.eq.enabled = false
			return nil
		}
		m.eq.enabled = true
		m.eq.preset = name
		if preset, ok := equalizer.PresetByName(name); ok {
			m.eq.gains = preset.Gains
		}
		return nil

	default:
		// a band
		band := m.eq.index - 2
		if band < 0 || band >= equalizer.Bands {
			return nil
		}
		m.eq.enabled = true
		m.eq.preset = equalizer.CustomPreset
		gain := m.eq.gains[band] + float64(dir)
		m.eq.gains[band] = equalizer.ClampGain(gain)
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
			break
		}
		if row, ok := layout.rowAt(msg.X, msg.Y); ok && row != m.eq.index {
			m.eq.index = row
		}

	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonMiddle {
			break
		}
		row, ok := layout.rowAt(msg.X, msg.Y)
		if !ok {
			m.eq = nil
			return tea.ClearScreen
		}
		m.eq.index = row
		switch row {
		case 0:
			m.eq.enabled = !m.eq.enabled
			return nil
		case 1:
			// the preset is changed by the halves of the row
			return m.eqChange(layout.valueDir(msg.X) > 0)
		default:
			band := row - 2
			if band >= 0 && band < equalizer.Bands {
				m.eq.enabled = true
				m.eq.preset = equalizer.CustomPreset
				gain := m.eq.gains[band] + float64(layout.valueDir(msg.X))
				m.eq.gains[band] = equalizer.ClampGain(gain)
				return nil
			}
		}
	}

	return nil
}
