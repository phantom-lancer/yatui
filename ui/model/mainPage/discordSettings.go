package mainpage

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/ui/style"
)

// discordRow is one clickable line of the Discord panel.
type discordRow struct {
	label   string
	value   func() string
	change  func(dir int) tea.Cmd
	onEnter func() tea.Cmd
}

// discordSettingsModel is the Discord settings panel.
type discordSettingsModel struct {
	index  int
	rows   []discordRow
	x, y   int
	width  int
	height int
	valid  bool
}

// openDiscordSettings opens the panel.
func (m *Model) openDiscordSettings() tea.Cmd {
	cfg := discordConfig()

	rows := []discordRow{
		{
			label: "Включить",
			value: func() string { return onOff(cfg.Enabled) },
			change: func(dir int) tea.Cmd {
				cfg.Enabled = !cfg.Enabled
				return m.discordApply()
			},
		},
		{
			label: "Что показывать",
			value: func() string {
				switch cfg.StatusType {
				case 1:
					return "артист"
				case 2:
					return "название трека"
				}
				return "имя"
			},
			change: func(dir int) tea.Cmd {
				cfg.StatusType = (cfg.StatusType + dir + 3) % 3
				return m.discordApply()
			},
		},
		{
			label: "Язык статуса",
			value: func() string {
				if cfg.Language == "ru" {
					return "русский"
				}
				return "english"
			},
			change: func(dir int) tea.Cmd {
				if cfg.Language == "ru" {
					cfg.Language = "en"
				} else {
					cfg.Language = "ru"
				}
				return m.discordApply()
			},
		},
		{
			label: "Альбом",
			value: func() string { return onOff(cfg.ShowAlbum) },
			change: func(dir int) tea.Cmd {
				cfg.ShowAlbum = !cfg.ShowAlbum
				return m.discordApply()
			},
		},
		{
			label: "Версия трека",
			value: func() string { return onOff(cfg.ShowVersion) },
			change: func(dir int) tea.Cmd {
				cfg.ShowVersion = !cfg.ShowVersion
				return m.discordApply()
			},
		},
		{
			label: "Маленькая иконка",
			value: func() string { return onOff(cfg.ShowSmallIcon) },
			change: func(dir int) tea.Cmd {
				cfg.ShowSmallIcon = !cfg.ShowSmallIcon
				return m.discordApply()
			},
		},
		{
			label: "Кнопки",
			value: func() string { return onOff(cfg.ShowButtons) },
			change: func(dir int) tea.Cmd {
				cfg.ShowButtons = !cfg.ShowButtons
				return m.discordApply()
			},
		},
		{
			label: "Объединить ссылки",
			value: func() string { return onOff(cfg.MergeLinks) },
			change: func(dir int) tea.Cmd {
				cfg.MergeLinks = !cfg.MergeLinks
				return m.discordApply()
			},
		},
		{
			label: "Убирать при паузе",
			value: func() string {
				if cfg.AfkTimeout == 0 {
					return "сразу"
				}
				return fmt.Sprintf("%d мин", cfg.AfkTimeout)
			},
			change: func(dir int) tea.Cmd {
				steps := []int{0, 5, 15, 30, 60}
				idx := 0
				for i, v := range steps {
					if v == cfg.AfkTimeout {
						idx = i
					}
				}
				idx = (idx + dir + len(steps)) % len(steps)
				cfg.AfkTimeout = steps[idx]
				return m.discordApply()
			},
		},
		{
			label: "Закрыть",
			value: func() string { return "" },
			change: func(dir int) tea.Cmd {
				m.discordSettings = nil
				return tea.ClearScreen
			},
		},
	}

	m.discordSettings = &discordSettingsModel{rows: rows}
	return tea.ClearScreen
}

// discordApply saves the settings and refreshes the card right away.
func (m *Model) discordApply() tea.Cmd {
	m.saveConfig()
	m.discordClear()
	m.discordRefresh()
	return nil
}

// discordSettingsView renders the panel.
func (m *Model) discordSettingsView() string {
	ds := m.discordSettings
	if ds == nil {
		return ""
	}

	labelWidth := 0
	for _, row := range ds.rows {
		if w := lipgloss.Width(row.label); w > labelWidth {
			labelWidth = w
		}
	}

	lines := make([]string, 0, len(ds.rows)+2)
	for i, row := range ds.rows {
		line := padRightCells(row.label, labelWidth) + "  "
		if row.label != "Закрыть" {
			line += "‹ " + row.value() + " ›"
		} else {
			line += row.value()
		}

		rowStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(style.NormalTextColor)
		if i == ds.index {
			rowStyle = rowStyle.Foreground(style.InactiveTextColor).Background(style.AccentColor)
		}
		lines = append(lines, rowStyle.Render(line))
	}

	box := style.DialogBoxStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		style.DialogTitleStyle.Render("Discord RPC"),
		lipgloss.JoinVertical(lipgloss.Left, lines...),
	))

	return lipgloss.JoinVertical(
		lipgloss.Center,
		box,
		style.DialogHelpStyle.Render("↑/↓ — выбор, ←/→ — изменить, esc — закрыть"),
	)
}

// placeDiscordSettings draws the panel and remembers the row geometry.
func (m *Model) placeDiscordSettings() string {
	view := m.discordSettingsView()
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

	m.discordSettings.x = x
	m.discordSettings.y = y
	m.discordSettings.width = width
	m.discordSettings.height = height
	m.discordSettings.valid = true

	return placed
}

// discordRowAt returns the row under the cursor (title and borders are not
// rows).
func (m *Model) discordRowAt(x, y int) int {
	ds := m.discordSettings
	if ds == nil || !ds.valid {
		return -1
	}
	if x < ds.x || x >= ds.x+ds.width || y < ds.y || y >= ds.y+ds.height {
		return -1
	}
	// border(1) + padding(1) + title(1) + title margin(1)
	row := y - (ds.y + 4)
	if row < 0 || row >= len(ds.rows) {
		return -1
	}
	return row
}

// discordSettingsControl handles keys and mouse while the panel is open.
func (m *Model) discordSettingsControl(message tea.Msg) (tea.Cmd, bool) {
	ds := m.discordSettings
	if ds == nil {
		return nil, false
	}

	switch msg := message.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.discordSettings = nil
			return tea.ClearScreen, true
		case "up":
			if ds.index > 0 {
				ds.index--
			}
			return nil, true
		case "down":
			if ds.index < len(ds.rows)-1 {
				ds.index++
			}
			return nil, true
		case "left", "right", "enter", " ":
			dir := 0
			if msg.String() == "left" {
				dir = -1
			} else if msg.String() == "right" {
				dir = 1
			}
			row := ds.rows[ds.index]
			if dir == 0 && row.onEnter != nil {
				return row.onEnter(), true
			}
			if row.change != nil {
				return row.change(dir), true
			}
			return nil, true
		}
		return nil, true

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionMotion {
			if row := m.discordRowAt(msg.X, msg.Y); row >= 0 {
				ds.index = row
			}
			return nil, true
		}
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			row := m.discordRowAt(msg.X, msg.Y)
			if row < 0 {
				return nil, true
			}
			ds.index = row
			dir := 0
			if msg.X >= ds.x+ds.width/2 {
				dir = 1
			}
			if ds.rows[row].change != nil {
				return ds.rows[row].change(dir), true
			}
		}
		return nil, true
	}

	return nil, false
}

// discordSettingItem is the settings row that opens the panel.
func (m *Model) discordSettingItem() settingItem {
	return settingItem{
		label: "Discord RPC",
		kind:  settingChoice,
		value: func() string {
			if cfg := discordConfig(); !cfg.Enabled {
				return "выкл"
			}
			return "настроить"
		},
		apply: func(dir int) tea.Cmd { return m.openDiscordSettings() },
	}
}
