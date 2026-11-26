package mainpage

import (
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/ui/style"
)

type settingKind int

const (
	settingBool settingKind = iota
	settingNumber
	settingChoice
	settingInfo
	settingSection
	settingSlider
)

// settingItem is a single row of the settings dialog. dir is -1 for "left",
// 0 for "activate" and 1 for "right".
type settingItem struct {
	label      string
	kind       settingKind
	value      func() string
	apply      func(dir int) tea.Cmd
	sliderMin  int
	sliderMax  int
	sliderStep int
	sliderGet  func() int
	sliderSet  func(int) tea.Cmd
}

type settingsModel struct {
	index       int
	width       int
	items       []settingItem
	layout      settingsLayout
	dragging    bool
	draggingRow int
}

// settingsLayout is where the rendered dialog ended up on the screen, so the
// mouse can be mapped back to the rows.
type settingsLayout struct {
	x, y          int
	width, height int
	// rowX is the first cell of a row, rowY the first row
	rowX, rowY int
	// rowWidth is the width of the row content, labelWidth the width of the
	// label column
	rowWidth, labelWidth int
	rows                 int
	valid                bool
}

// rowAt returns the index of the row under the given screen coordinates.
func (l settingsLayout) rowAt(x, y int) (int, bool) {
	if !l.valid || x < l.x || x >= l.x+l.width || y < l.rowY {
		return 0, false
	}
	if row := y - l.rowY; row < l.rows {
		return row, true
	}
	return 0, false
}

// valueDir reports which part of the row is under the cursor: -1 for the left
// half of the value, 1 for the right one and 0 for the rest of the row.
func (l settingsLayout) valueDir(x, row int) int {
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

// placeSettings draws the dialog in the middle of the screen and remembers
// where every row landed.
func (m *Model) placeSettings() string {
	view := m.settings.view()
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
	m.settings.layout = settingsLayout{
		x:          x,
		y:          y,
		width:      width,
		height:     height,
		rowX:       x + box.GetBorderLeftSize() + box.GetPaddingLeft() + 1,
		rowY:       y + box.GetBorderTopSize() + box.GetPaddingTop() + 2,
		rowWidth:   m.settings.width,
		labelWidth: m.settings.labelWidth(),
		rows:       len(m.settings.items),
		valid:      len(m.settings.items) > 0,
	}

	return placed
}

// settingsMouse handles the mouse while the settings dialog is open. Only the
// dialog is interactive: everything behind it is ignored.
func (m *Model) settingsMouse(msg tea.MouseMsg) tea.Cmd {
	if m.settings == nil {
		return nil
	}
	layout := m.settings.layout

	switch msg.Action {
	case tea.MouseActionMotion:
		if m.settings.dragging && m.settings.draggingRow >= 0 && m.settings.draggingRow < len(m.settings.items) {
			m.settings.index = m.settings.draggingRow
			return tea.Batch(tea.ClearScreen, m.setSettingsSliderAt(m.settings.draggingRow, msg.X))
		}
		if msg.Button != tea.MouseButtonNone {
			break
		}
		if row, ok := layout.rowAt(msg.X, msg.Y); ok && row != m.settings.index {
			m.settings.index = row
		}

	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonMiddle {
			break
		}

		row, ok := layout.rowAt(msg.X, msg.Y)
		if !ok {
			m.isSettingsActive = false
			m.settings = nil
			return tea.ClearScreen
		}

		m.settings.index = row
		if m.settings.items[row].kind == settingSlider && msg.Button == tea.MouseButtonLeft {
			m.settings.dragging = true
			m.settings.draggingRow = row
			return tea.Batch(tea.ClearScreen, m.setSettingsSliderAt(row, msg.X))
		}
		return tea.Batch(tea.ClearScreen, m.settings.change(layout.valueDir(msg.X, row)))

	case tea.MouseActionRelease:
		if m.settings.dragging {
			m.settings.dragging = false
			m.settings.draggingRow = 0
			return tea.ClearScreen
		}
	}

	return nil
}

func (s *settingsModel) up() {
	s.move(-1)
}

func (s *settingsModel) down() {
	s.move(1)
}

// Section headings are visual separators, not selectable settings.
func (s *settingsModel) move(direction int) {
	if len(s.items) == 0 {
		return
	}
	for range s.items {
		s.index = (s.index + direction + len(s.items)) % len(s.items)
		if s.items[s.index].kind != settingSection {
			return
		}
	}
}

func (s *settingsModel) change(dir int) tea.Cmd {
	if s.index < 0 || s.index >= len(s.items) {
		return nil
	}

	item := s.items[s.index]
	if item.kind == settingSlider && item.sliderSet != nil && item.sliderGet != nil {
		step := item.sliderStep
		if step < 1 {
			step = 1
		}
		value := item.sliderGet() + dir*step
		if value < item.sliderMin {
			value = item.sliderMin
		}
		if value > item.sliderMax {
			value = item.sliderMax
		}
		return item.sliderSet(value)
	}
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

		// the value of the selected row is framed with arrows: they show
		// where the left and the right mouse buttons are, and the halves of
		// the value area work the way the arrows point
		valueCell := padLeft(value, valueWidth)
		if item.kind == settingSection {
			valueCell = strings.Repeat(" ", valueWidth)
		}
		if i == s.index && item.kind != settingSection && item.kind != settingSlider && valueWidth >= 6 {
			valueCell = "‹ " + padLeft(value, valueWidth-4) + " ›"
		}

		if item.kind == settingSlider && item.sliderGet != nil {
			valueCell = padRight(renderSettingsSlider(item.sliderGet(), item.sliderMin, item.sliderMax, valueWidth), valueWidth)
		}
		// the row is laid out by hand: every line is exactly s.width cells
		// wide, so a long value can never wrap and break the grid
		line := padRight(item.label, labelWidth) + "  " + valueCell

		rowStyle := lipgloss.NewStyle().Padding(0, 1)
		if item.kind == settingSection {
			rowStyle = rowStyle.Foreground(style.AccentColor).Bold(true)
		} else if i == s.index {
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
		style.DialogHelpStyle.Render("↑/↓, мышь — выбор, ←/→ — изменить, esc — выход"),
	)
}

func renderSettingsSlider(value, minValue, maxValue, width int) string {
	if width <= 0 {
		return ""
	}
	if maxValue <= minValue {
		return padRight(fmt.Sprintf("%d", value), width)
	}
	barWidth := width - 7
	if barWidth < 3 {
		barWidth = 3
	}
	if barWidth > 24 {
		barWidth = 24
	}
	value = max(minValue, min(maxValue, value))
	filled := int(math.Round(float64(value-minValue) / float64(maxValue-minValue) * float64(barWidth-1)))
	bar := "[" + strings.Repeat("=", filled) + ">" + strings.Repeat("-", barWidth-filled-1) + "]"
	return fmt.Sprintf("%s %3d%%", bar, value)
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

func (m *Model) setSettingsSliderAt(row, x int) tea.Cmd {
	if m.settings == nil || row < 0 || row >= len(m.settings.items) {
		return nil
	}
	item := m.settings.items[row]
	if item.kind != settingSlider || item.sliderSet == nil {
		return nil
	}
	layout := m.settings.layout
	valueX := layout.rowX + layout.labelWidth + 2
	valueWidth := layout.rowWidth - layout.labelWidth - 2
	if valueWidth <= 1 {
		return nil
	}
	fraction := float64(x-valueX) / float64(valueWidth-1)
	fraction = math.Max(0, math.Min(1, fraction))
	value := item.sliderMin + int(math.Round(fraction*float64(item.sliderMax-item.sliderMin)))
	step := max(1, item.sliderStep)
	value = item.sliderMin + int(math.Round(float64(value-item.sliderMin)/float64(step)))*step
	value = max(item.sliderMin, min(item.sliderMax, value))
	return item.sliderSet(value)
}

// toggleSettings opens or closes the settings dialog.
func (m *Model) toggleSettings() {
	if m.isSettingsActive {
		m.isSettingsActive = false
		m.settings = nil
		return
	}

	width := 56
	if m.width > 0 && width > m.width-6 {
		width = m.width - 6
	}
	if width < 28 {
		width = 28
	}
	m.settings = &settingsModel{width: width, items: m.buildSettings()}
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

	keypress := msg.String()

	var cmd tea.Cmd
	switch {
	case keypress == "esc":
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
	items := []settingItem{
		{label: "ВОСПРОИЗВЕДЕНИЕ", kind: settingSection},
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
				return fmt.Sprintf("%d (после перезапуска)", max(180, min(500, int(config.Current.BufferSize))))
			},
			apply: func(dir int) tea.Cmd {
				buffer := config.Current.BufferSize + float64(dir)*10
				if buffer < 180 {
					buffer = 180
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
				if config.Current.Normalization == nil || !config.Current.Normalization.Enabled {
					return "выкл"
				}
				// the measured correction of the current track is shown
				// right here: it makes the feature visible
				if gain := m.tracker.LastGain(); math.Abs(gain) > 0.05 {
					return fmt.Sprintf("вкл (%.1f дБ)", gain)
				}
				return "вкл"
			},
			apply: func(dir int) tea.Cmd {
				if config.Current.Normalization == nil {
					config.Current.Normalization = &config.Normalization{}
				}
				config.Current.Normalization.Enabled = !config.Current.Normalization.Enabled
				m.saveConfig()
				m.tracker.SetNormalization(config.Current.Normalization.Enabled)
				m.showToast("Нормализация: " + onOff(config.Current.Normalization.Enabled))
				return nil
			},
		},
		{
			// the gain is measured by decoding the track: off is the lightest
			// mode for the CPU
			label: "Точность нормализации",
			kind:  settingBool,
			value: func() string {
				if config.Current.Normalization == nil || !config.Current.Normalization.Fast {
					return "точно (10 с трека)"
				}
				return "быстро (1 с трека, экономно)"
			},
			apply: func(dir int) tea.Cmd {
				if config.Current.Normalization == nil {
					config.Current.Normalization = &config.Normalization{}
				}
				config.Current.Normalization.Fast = !config.Current.Normalization.Fast
				m.saveConfig()
				m.tracker.SetFastNormalization(config.Current.Normalization.Fast)
				if config.Current.Normalization.Fast {
					m.showToast("Нормализация: быстрый замер")
				} else {
					m.showToast("Нормализация: точный замер")
				}
				return nil
			},
		},
		{label: "ВНЕШНИЙ ВИД", kind: settingSection},
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
				cmd := m.cycleTheme()
				// the row list depends on the theme (the rgb speed and the
				// dev reset are only listed there), so rebuild it in place
				if m.settings != nil {
					index := m.settings.index
					m.settings.items = m.buildSettings()
					if index >= len(m.settings.items) {
						index = len(m.settings.items) - 1
					}
					m.settings.index = index
				}
				return cmd
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
			label: "Визуализатор «Моей волны» (выкл / панель / фон)",
			kind:  settingBool,
			value: func() string {
				return vizStatus()
			},
			apply: func(dir int) tea.Cmd {
				return m.cycleVisualizer()
			},
		},
		{
			label: "Экономия ресурсов",
			kind:  settingBool,
			value: func() string {
				return onOff(config.Current.ResourceSaver)
			},
			apply: func(dir int) tea.Cmd {
				config.Current.ResourceSaver = !config.Current.ResourceSaver
				m.saveConfig()
				return m.applyResourceSaving()
			},
		},
		{
			label: "Классический вид эквалайзера",
			kind:  settingBool,
			value: func() string {
				if config.Current.Equalizer == nil || !config.Current.Equalizer.ClassicView {
					return "выкл (графический)"
				}
				return "вкл (10 полос)"
			},
			apply: func(dir int) tea.Cmd {
				if config.Current.Equalizer == nil {
					config.Current.Equalizer = &config.Equalizer{}
				}
				config.Current.Equalizer.ClassicView = !config.Current.Equalizer.ClassicView
				m.saveConfig()
				m.applySavedEQ()
				if m.eq != nil {
					m.eq.graphic = !config.Current.Equalizer.ClassicView
					m.eq.index = -1
				}
				if config.Current.Equalizer.ClassicView {
					m.showToast("Эквалайзер: классический вид")
				} else {
					m.showToast("Эквалайзер: графический вид")
				}
				return nil
			},
		},
		{
			label: "Офлайн-режим",
			kind:  settingBool,
			value: func() string {
				return offlineStatus()
			},
			apply: func(dir int) tea.Cmd {
				return m.applyOfflineMode()
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
		{label: "СЕРВИСЫ И ОБНОВЛЕНИЯ", kind: settingSection},
		{
			label: "Предзагрузка следующего трека",
			kind:  settingBool,
			value: func() string { return onOff(config.Current.PreloadNextTrack) },
			apply: func(dir int) tea.Cmd {
				config.Current.PreloadNextTrack = !config.Current.PreloadNextTrack
				if !config.Current.PreloadNextTrack {
					m.clearPreload()
				} else {
					m.preloadNextTrack()
				}
				m.saveConfig()
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
			label: "Обновления",
			kind:  settingChoice,
			value: func() string {
				return m.updateStatusText()
			},
			apply: func(dir int) tea.Cmd {
				return m.checkUpdatesCmd()
			},
		},
		m.discordSettingItem(),
		{
			label: "Конфиг",
			kind:  settingInfo,
			value: func() string {
				return config.Path()
			},
			apply: func(dir int) tea.Cmd {
				return openConfig()
			},
		},
	}

	if winterActive() {
		winterItems := []settingItem{
			{
				label: "Скорость снега", kind: settingSlider,
				sliderMin: 10, sliderMax: 100, sliderStep: 5,
				sliderGet: func() int { return config.Current.WinterSpeed },
				sliderSet: func(value int) tea.Cmd {
					config.Current.WinterSpeed = max(10, min(100, value))
					config.SaveLater()
					m.idleFrameValid = false
					return nil
				},
			},
			{
				label: "Плотность снега", kind: settingSlider,
				sliderMin: 0, sliderMax: 100, sliderStep: 5,
				sliderGet: func() int { return config.Current.WinterDensity },
				sliderSet: func(value int) tea.Cmd {
					config.Current.WinterDensity = max(0, min(100, value))
					config.SaveLater()
					m.idleFrameValid = false
					return nil
				},
			},
			{
				label: "Гирлянда", kind: settingBool,
				value: func() string { return onOff(config.Current.WinterGarland) },
				apply: func(dir int) tea.Cmd { return m.toggleWinterGarland() },
			},
		}
		at := 0
		for i, item := range items {
			if item.label == "Тема" {
				at = i + 1
				break
			}
		}
		items = append(items, make([]settingItem, len(winterItems))...)
		copy(items[at+len(winterItems):], items[at:len(items)-len(winterItems)])
		copy(items[at:], winterItems)
	}

	if m.updateRelease != nil {
		items = append(items, settingItem{
			label: "Установить обновление",
			kind:  settingChoice,
			value: func() string {
				if m.updateDownloading {
					return "скачивание..."
				}
				return m.updateRelease.TagName
			},
			apply: func(dir int) tea.Cmd {
				return m.downloadUpdateCmd()
			},
		})
	}

	// the whole animated theme is configured in its own panel: one row here
	// instead of its knobs mixed into the general list
	if rgbActive() {
		items = append(items, m.rgbSettingItem())
	}

	return items
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

// openConfig opens the config file in Notepad so the user can edit it by hand
// (for example the Last.fm api key and secret).
func openConfig() tea.Cmd {
	path := config.Path()
	return func() tea.Msg {
		cmd := exec.Command("notepad.exe", path)
		if err := cmd.Start(); err != nil {
			log.Print(log.LVL_WARNIGN, "failed to open config: %s", err)
		}
		return nil
	}
}
