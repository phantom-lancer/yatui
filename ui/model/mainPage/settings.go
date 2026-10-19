package mainpage

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/dece2183/yamusic-tui/config"
	"github.com/dece2183/yamusic-tui/log"
	"github.com/dece2183/yamusic-tui/ui/style"
)

type settingKind int

const (
	settingBool settingKind = iota
	settingNumber
	settingChoice
	settingInfo
)

// settingItem is a single row of the settings dialog. dir is -1 for "left",
// 0 for "activate" and 1 for "right".
type settingItem struct {
	label string
	kind  settingKind
	value func() string
	apply func(dir int) tea.Cmd
}

type settingsModel struct {
	index int
	width int
	items []settingItem
}

func (s *settingsModel) up() {
	if len(s.items) == 0 {
		return
	}
	s.index--
	if s.index < 0 {
		s.index = len(s.items) - 1
	}
}

func (s *settingsModel) down() {
	if len(s.items) == 0 {
		return
	}
	s.index++
	if s.index >= len(s.items) {
		s.index = 0
	}
}

func (s *settingsModel) change(dir int) tea.Cmd {
	if s.index < 0 || s.index >= len(s.items) {
		return nil
	}

	item := s.items[s.index]
	if item.apply == nil {
		return nil
	}

	// booleans and choices react to any of the left/right/enter keys
	if dir == 0 && (item.kind == settingBool || item.kind == settingChoice) {
		dir = 1
	}

	return item.apply(dir)
}

func (s *settingsModel) view() string {
	labelWidth := s.labelWidth()
	valueWidth := s.width - labelWidth - 2
	if valueWidth < 4 {
		valueWidth = 4
	}

	rows := make([]string, 0, len(s.items))
	for i, item := range s.items {
		value := ""
		if item.value != nil {
			value = item.value()
		}

		// the row is laid out by hand: every line is exactly s.width cells
		// wide, so a long value can never wrap and break the grid
		line := padRight(item.label, labelWidth) + "  " + padLeft(value, valueWidth)

		rowStyle := lipgloss.NewStyle().Padding(0, 1)
		if i == s.index {
			rowStyle = rowStyle.
				Foreground(style.InactiveTextColor).
				Background(style.AccentColor)
		} else {
			rowStyle = rowStyle.Foreground(style.NormalTextColor)
		}
		rows = append(rows, rowStyle.Render(line))
	}

	box := style.DialogBoxStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		style.DialogTitleStyle.Render("Настройки"),
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		style.DialogHelpStyle.Render("↑/↓ — выбрать, ←/→ — изменить, esc — закрыть"),
	)
}

// padRight pads (or truncates) a string to the given number of cells.
func padRight(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if w := ansi.StringWidth(value); w > width {
		return ansi.Truncate(value, width, "…")
	} else if w < width {
		return value + strings.Repeat(" ", width-w)
	}
	return value
}

// padLeft right-aligns a string in the given number of cells. Long values are
// truncated from the left, so the end of a path or a number stays visible.
func padLeft(value string, width int) string {
	if width <= 0 {
		return ""
	}
	w := ansi.StringWidth(value)
	if w > width {
		value = ansi.TruncateLeft(value, w-width+1, "…")
		w = ansi.StringWidth(value)
	}
	if w < width {
		value = strings.Repeat(" ", width-w) + value
	}
	return value
}

func (s *settingsModel) labelWidth() int {
	width := 0
	for _, item := range s.items {
		if w := lipgloss.Width(item.label); w > width {
			width = w
		}
	}
	return width
}

// toggleSettings opens or closes the settings dialog.
func (m *Model) toggleSettings() {
	if m.isSettingsActive {
		m.isSettingsActive = false
		m.settings = nil
		return
	}

	m.settings = &settingsModel{width: 44, items: m.buildSettings()}
	m.isSettingsActive = true
}

// settingsControl handles the keys while the dialog is open.
func (m *Model) settingsControl(message tea.Msg) (tea.Cmd, bool) {
	if !m.isSettingsActive || m.settings == nil {
		return nil, false
	}

	// only keys are captured: playback events must keep flowing
	msg, ok := message.(tea.KeyMsg)
	if !ok {
		return nil, false
	}

	controls := config.Current.Controls
	keypress := msg.String()

	var cmd tea.Cmd
	switch {
	case keypress == "esc", controls.Settings.Contains(keypress):
		m.isSettingsActive = false
		m.settings = nil
	case keypress == "up":
		m.settings.up()
	case keypress == "down":
		m.settings.down()
	case keypress == "left":
		cmd = m.settings.change(-1)
	case keypress == "right":
		cmd = m.settings.change(1)
	case keypress == "enter", keypress == " ":
		cmd = m.settings.change(0)
	}

	return tea.Batch(tea.ClearScreen, cmd), true
}

// buildSettings describes every setting the dialog can change. Values are
// read back from the config on every paint, so the rows always show the
// current state.
func (m *Model) buildSettings() []settingItem {
	return []settingItem{
		{
			label: "Громкость",
			kind:  settingNumber,
			value: func() string {
				return fmt.Sprintf("%d%%", int(config.Current.Volume*100+0.5))
			},
			apply: func(dir int) tea.Cmd {
				step := config.Current.VolumeStep
				if step <= 0 {
					step = 0.05
				}
				m.tracker.SetVolumeTo(config.Current.Volume + float64(dir)*step)
				return nil
			},
		},
		{
			label: "Шаг громкости",
			kind:  settingNumber,
			value: func() string {
				return fmt.Sprintf("%d%%", int(config.Current.VolumeStep*100+0.5))
			},
			apply: func(dir int) tea.Cmd {
				step := config.Current.VolumeStep + float64(dir)*0.01
				if step < 0.01 {
					step = 0.01
				} else if step > 0.25 {
					step = 0.25
				}
				config.Current.VolumeStep = step
				m.saveConfig()
				return nil
			},
		},
		{
			label: "Буфер, мс",
			kind:  settingNumber,
			value: func() string {
				return fmt.Sprintf("%d (после перезапуска)", int(config.Current.BufferSize))
			},
			apply: func(dir int) tea.Cmd {
				buffer := config.Current.BufferSize + float64(dir)*10
				if buffer < 40 {
					buffer = 40
				} else if buffer > 500 {
					buffer = 500
				}
				config.Current.BufferSize = buffer
				m.saveConfig()
				return nil
			},
		},
		{
			label: "Перемотка, с",
			kind:  settingNumber,
			value: func() string {
				return fmt.Sprintf("%d", int(config.Current.RewindDuration))
			},
			apply: func(dir int) tea.Cmd {
				seconds := config.Current.RewindDuration + float64(dir)
				if seconds < 1 {
					seconds = 1
				} else if seconds > 30 {
					seconds = 30
				}
				m.tracker.SetRewind(time.Duration(seconds) * time.Second)
				return nil
			},
		},
		{
			label: "Нормализация",
			kind:  settingBool,
			value: func() string {
				return onOff(config.Current.Normalization != nil && config.Current.Normalization.Enabled)
			},
			apply: func(dir int) tea.Cmd {
				if config.Current.Normalization == nil {
					config.Current.Normalization = &config.Normalization{}
				}
				config.Current.Normalization.Enabled = !config.Current.Normalization.Enabled
				m.saveConfig()
				m.showToast("Нормализация: " + onOff(config.Current.Normalization.Enabled))
				return nil
			},
		},
		{
			label: "Тема",
			kind:  settingChoice,
			value: func() string {
				if config.Current.Style == nil || config.Current.Style.Theme == "" {
					return "по конфигу"
				}
				return config.Current.Style.Theme
			},
			apply: func(dir int) tea.Cmd {
				m.cycleTheme()
				return nil
			},
		},
		{
			label: "Лирики",
			kind:  settingBool,
			value: func() string {
				return onOff(m.tracker.Lyrics())
			},
			apply: func(dir int) tea.Cmd {
				m.tracker.SetLirycs(!m.tracker.Lyrics())
				return nil
			},
		},
		{
			label: "Кэш треков",
			kind:  settingChoice,
			value: func() string {
				switch config.Current.CacheTracks {
				case config.CACHE_ALL:
					return "всё"
				case config.CACHE_LIKED_ONLY:
					return "только лайки"
				default:
					return "выкл"
				}
			},
			apply: func(dir int) tea.Cmd {
				config.Current.CacheTracks = (config.Current.CacheTracks + 1) % 3
				m.saveConfig()
				m.showToast("Кэш: " + cacheName(config.Current.CacheTracks) + " (после перезапуска)")
				return nil
			},
		},
		{
			label: "Last.fm",
			kind:  settingChoice,
			value: func() string {
				return m.lastfmStatus()
			},
			apply: func(dir int) tea.Cmd {
				return m.lastfmSetupCmd()
			},
		},
		{
			label: "Подавлять ошибки",
			kind:  settingBool,
			value: func() string {
				return onOff(config.Current.SuppressErrors)
			},
			apply: func(dir int) tea.Cmd {
				config.Current.SuppressErrors = !config.Current.SuppressErrors
				m.saveConfig()
				return nil
			},
		},
		{
			label: "Конфиг",
			kind:  settingInfo,
			value: func() string {
				return config.Path()
			},
		},
	}
}

func cacheName(cache config.CacheType) string {
	switch cache {
	case config.CACHE_ALL:
		return "всё"
	case config.CACHE_LIKED_ONLY:
		return "только лайки"
	default:
		return "выкл"
	}
}

func onOff(enabled bool) string {
	if enabled {
		return "вкл"
	}
	return "выкл"
}

// saveConfig writes the config and reports a failure, which is otherwise
// invisible to the user.
func (m *Model) saveConfig() {
	if err := config.Save(); err != nil {
		log.Print(log.LVL_WARNIGN, "failed to save config: %s", err)
		m.showToast("Не удалось сохранить конфиг")
	}
}
