package mainpage

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/ui/style"
)

// rgbFrameSteps are the smoothness choices of the animated theme, from the
// cheapest frame to the smoothest one.
var rgbFrameSteps = []int{500, 250, 200, 125, 100, 67, 40}

// rgbRow is one line of the RGB panel.
type rgbRow struct {
	label  string
	value  func() string
	change func(dir int) tea.Cmd
}

// rgbSettingsModel is the panel that keeps every knob of the animated theme in
// one place.
type rgbSettingsModel struct {
	index  int
	rows   []rgbRow
	x, y   int
	width  int
	height int
	valid  bool
}

// openRgbSettings opens the panel.
func (m *Model) openRgbSettings() tea.Cmd {
	rows := []rgbRow{
		{
			label: "Плавность",
			value: func() string {
				return rgbFrameLabel(rgbFrameMs())
			},
			change: func(dir int) tea.Cmd {
				if dir == 0 {
					dir = 1
				}
				return m.applyRgbFrame(stepRgbFrame(rgbFrameMs(), dir))
			},
		},
		{
			label: "Скорость",
			value: func() string {
				return rgbSpeedLabel(rgbSpeed())
			},
			change: func(dir int) tea.Cmd {
				if dir == 0 {
					dir = 1
				}
				return m.applyRgbSpeed(rgbSpeed() + dir)
			},
		},
		{
			label: "Как было",
			value: func() string {
				return "выключить и вернуть цвета"
			},
			change: func(dir int) tea.Cmd {
				m.themeReset()
				m.rgbSettings = nil
				return tea.ClearScreen
			},
		},
		{
			label: "Закрыть",
			value: func() string {
				return ""
			},
			change: func(dir int) tea.Cmd {
				m.rgbSettings = nil
				return tea.ClearScreen
			},
		},
	}

	m.rgbSettings = &rgbSettingsModel{rows: rows}
	return tea.ClearScreen
}

// stepRgbFrame returns the next frame of the smoothness list.
func stepRgbFrame(current, dir int) int {
	index := 0
	for i, ms := range rgbFrameSteps {
		if ms == current {
			index = i
		}
	}
	index = (index + dir + len(rgbFrameSteps)) % len(rgbFrameSteps)
	return rgbFrameSteps[index]
}

// rgbSpeedLabel describes the speed slider the way the user sees it.
func rgbSpeedLabel(speed int) string {
	switch {
	case speed <= 2:
		return "медленно"
	case speed <= 4:
		return "спокойно"
	case speed <= 6:
		return "обычно"
	case speed <= 8:
		return "быстро"
	}
	return "психоделика"
}

// rgbSettingsView renders the panel.
func (m *Model) rgbSettingsView() string {
	rs := m.rgbSettings
	if rs == nil {
		return ""
	}

	labelWidth := 0
	for _, row := range rs.rows {
		if w := lipgloss.Width(row.label); w > labelWidth {
			labelWidth = w
		}
	}

	lines := make([]string, 0, len(rs.rows)+2)
	for i, row := range rs.rows {
		line := padRightCells(row.label, labelWidth) + "  "
		if row.label != "Закрыть" {
			line += "‹ " + row.value() + " ›"
		} else {
			line += row.value()
		}

		rowStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(style.NormalTextColor)
		if i == rs.index {
			rowStyle = rowStyle.Foreground(style.InactiveTextColor).Background(style.AccentColor)
		}
		lines = append(lines, rowStyle.Render(line))
	}

	box := style.DialogBoxStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		style.DialogTitleStyle.Render("RGB-тема"),
		lipgloss.JoinVertical(lipgloss.Left, lines...),
	))

	return lipgloss.JoinVertical(
		lipgloss.Center,
		box,
		style.DialogHelpStyle.Render("↑/↓ — выбор, ←/→ — изменить, esc — закрыть"),
	)
}

// placeRgbSettings draws the panel and remembers the row geometry.
func (m *Model) placeRgbSettings() string {
	view := m.rgbSettingsView()
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

	m.rgbSettings.x = x
	m.rgbSettings.y = y
	m.rgbSettings.width = width
	m.rgbSettings.height = height
	m.rgbSettings.valid = true

	return placed
}

// rgbRowAt returns the row under the cursor (title and borders are not rows).
func (m *Model) rgbRowAt(x, y int) int {
	rs := m.rgbSettings
	if rs == nil || !rs.valid {
		return -1
	}
	if x < rs.x || x >= rs.x+rs.width || y < rs.y || y >= rs.y+rs.height {
		return -1
	}
	// border(1) + padding(1) + title(1) + title margin(1)
	row := y - (rs.y + 4)
	if row < 0 || row >= len(rs.rows) {
		return -1
	}
	return row
}

// rgbSettingsControl handles keys and mouse while the panel is open.
func (m *Model) rgbSettingsControl(message tea.Msg) (tea.Cmd, bool) {
	rs := m.rgbSettings
	if rs == nil {
		return nil, false
	}

	switch msg := message.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.rgbSettings = nil
			return tea.ClearScreen, true
		case "up":
			if rs.index > 0 {
				rs.index--
			}
			return nil, true
		case "down":
			if rs.index < len(rs.rows)-1 {
				rs.index++
			}
			return nil, true
		case "left", "right", "enter", " ":
			dir := 0
			if msg.String() == "left" {
				dir = -1
			} else if msg.String() == "right" {
				dir = 1
			}
			row := rs.rows[rs.index]
			if row.change != nil {
				return row.change(dir), true
			}
			return nil, true
		}
		return nil, true

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionMotion {
			if row := m.rgbRowAt(msg.X, msg.Y); row >= 0 {
				rs.index = row
			}
			return nil, true
		}
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			row := m.rgbRowAt(msg.X, msg.Y)
			if row < 0 {
				return nil, true
			}
			rs.index = row
			dir := 0
			if msg.X >= rs.x+rs.width/2 {
				dir = 1
			}
			if rs.rows[row].change != nil {
				return rs.rows[row].change(dir), true
			}
		}
		return nil, true
	}

	return nil, false
}

// rgbSettingItem is the settings row that opens the panel. The whole animated
// theme is configured there, so the general list only gets this one entry.
func (m *Model) rgbSettingItem() settingItem {
	return settingItem{
		label: "RGB-тема",
		kind:  settingChoice,
		value: func() string {
			return rgbFrameLabel(rgbFrameMs())
		},
		apply: func(dir int) tea.Cmd { return m.openRgbSettings() },
	}
}
