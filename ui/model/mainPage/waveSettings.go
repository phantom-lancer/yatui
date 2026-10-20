package mainpage

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
	"github.com/phantom-lancer/yatui/ui/style"
)

// _WAVE_SETTINGS_WIDTH is the width of the window with the wave settings.
const _WAVE_SETTINGS_WIDTH = 40

// waveOption is one option of a group of the wave settings.
type waveOption struct {
	// value is the value the server knows the option by.
	value string
	// name is the name of the option shown to the user.
	name string
	// seed is the seed that switches the wave to this option.
	seed string
	// unspecified marks the option that means "no restriction at all".
	unspecified bool
}

// waveGroup is a group of the options of the wave settings.
type waveGroup struct {
	key     string
	title   string
	options []waveOption
	// index is the option that is chosen right now.
	index int
}

// waveSettingsModel is the window with the settings of the main wave.
type waveSettingsModel struct {
	groups  []waveGroup
	index   int
	loading bool
	layout  waveLayout
}

// waveLayout is where the rows of the window ended up on the screen.
type waveLayout struct {
	x, y          int
	width, height int
	rowX, rowY    int
	rowWidth      int
	labelWidth    int
	rows          int
	valid         bool
}

// rowAt returns the index of the row under the given screen coordinates.
func (l waveLayout) rowAt(x, y int) (int, bool) {
	if !l.valid || x < l.x || x >= l.x+l.width || y < l.rowY {
		return 0, false
	}

	row := y - l.rowY
	if row < l.rows {
		return row, true
	}
	// the apply row is below an empty line
	if row == l.rows+1 {
		return l.rows, true
	}

	return 0, false
}

// valueDir reports which part of the value the cursor is on: -1 for the left
// half, 1 for the right one and 0 for the rest of the row.
func (l waveLayout) valueDir(x int) int {
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

// waveStationsMsg carries the stations the options of the wave are built of.
type waveStationsMsg struct {
	stations []api.StationDesc
}

// waveSessionMsg carries a new session of the main wave.
type waveSessionMsg struct {
	generation int64
	session    api.StationTracks
}

// waveGroups builds the groups of the wave settings out of the stations list:
// the activities and the options of the character, the mood and the language.
func waveGroups(stations []api.StationDesc) []waveGroup {
	var (
		activities []waveOption
		diversity  []waveOption
		mood       []waveOption
		language   []waveOption
	)

	seenActivity := make(map[string]bool)
	seenDiversity := make(map[string]bool)
	seenMood := make(map[string]bool)
	seenLanguage := make(map[string]bool)

	for _, station := range stations {
		id := station.Station.Id
		if id.Type == "activity" && id.Tag != "" && !seenActivity[id.Tag] {
			seenActivity[id.Tag] = true
			activities = append(activities, waveOption{
				value: id.Tag,
				name:  station.Station.Name,
				seed:  "activity:" + id.Tag,
			})
		}

		for _, option := range station.Station.Restrictions2.Diversity.PossibleValues {
			if option.Value == "" || seenDiversity[option.Value] {
				continue
			}
			seenDiversity[option.Value] = true
			diversity = append(diversity, waveOption{
				value:       option.Value,
				name:        option.Name,
				seed:        option.SerializedSeed,
				unspecified: option.Unspecified,
			})
		}

		for _, option := range station.Station.Restrictions2.MoodEnergy.PossibleValues {
			if option.Value == "" || seenMood[option.Value] {
				continue
			}
			seenMood[option.Value] = true
			mood = append(mood, waveOption{
				value:       option.Value,
				name:        option.Name,
				seed:        option.SerializedSeed,
				unspecified: option.Unspecified,
			})
		}

		for _, option := range station.Station.Restrictions2.Language.PossibleValues {
			if option.Value == "" || seenLanguage[option.Value] {
				continue
			}
			seenLanguage[option.Value] = true
			language = append(language, waveOption{
				value:       option.Value,
				name:        option.Name,
				seed:        option.SerializedSeed,
				unspecified: option.Unspecified,
			})
		}
	}

	// the wave without an activity is the usual one
	activities = append([]waveOption{{value: "", name: "Без занятия"}}, activities...)

	return []waveGroup{
		{key: "activity", title: "По занятию", options: activities},
		{key: "diversity", title: "По характеру", options: diversity},
		{key: "mood", title: "По настроению", options: mood},
		{key: "language", title: "По языку", options: language},
	}
}

// setStations builds the options of the window and marks the ones that are
// chosen in the config.
func (w *waveSettingsModel) setStations(stations []api.StationDesc) {
	w.groups = waveGroups(stations)

	wave := config.Current.Wave
	if wave == nil {
		return
	}

	for i := range w.groups {
		switch w.groups[i].key {
		case "activity":
			w.groups[i].selectValue(wave.Activity)
		case "diversity":
			w.groups[i].selectValue(wave.Diversity)
		case "mood":
			w.groups[i].selectValue(wave.MoodEnergy)
		case "language":
			w.groups[i].selectValue(wave.Language)
		}
	}
}

// selectValue marks the option with the given value. An unknown value means
// that nothing is chosen yet: the option without a restriction is used then.
func (g *waveGroup) selectValue(value string) {
	g.index = 0
	for i, option := range g.options {
		if option.unspecified {
			g.index = i
		}
		if option.value == value {
			g.index = i
			return
		}
	}
}

// labelWidth is the width of the column with the names of the groups.
func (w *waveSettingsModel) labelWidth() int {
	width := 0
	for _, group := range w.groups {
		if w := lipgloss.Width(group.title); w > width {
			width = w
		}
	}
	return width
}

// up moves the cursor to the previous row.
func (w *waveSettingsModel) up() {
	if w.index <= 0 {
		w.index = len(w.groups)
		return
	}
	w.index--
}

// down moves the cursor to the next row.
func (w *waveSettingsModel) down() {
	if w.index >= len(w.groups) {
		w.index = 0
		return
	}
	w.index++
}

// change switches the option of the selected group.
func (w *waveSettingsModel) change(dir int) {
	if dir == 0 || w.index < 0 || w.index >= len(w.groups) {
		return
	}

	group := &w.groups[w.index]
	if len(group.options) == 0 {
		return
	}

	group.index = (group.index + dir + len(group.options)) % len(group.options)
}

// option returns the option that is chosen in the given group.
func (w *waveSettingsModel) option(key string) waveOption {
	for _, group := range w.groups {
		if group.key != key {
			continue
		}
		if group.index < 0 || group.index >= len(group.options) {
			return waveOption{}
		}
		return group.options[group.index]
	}
	return waveOption{}
}

// waveView renders the window with the settings of the main wave.
func (m *Model) waveView() string {
	title := style.DialogTitleStyle.Render("Настроить Мою волну")

	if m.wave.loading {
		return style.DialogBoxStyle.Render(lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			style.DialogHelpStyle.Render("Загрузка..."),
		))
	}

	labelWidth := m.wave.labelWidth()
	valueWidth := _WAVE_SETTINGS_WIDTH - labelWidth - 2
	if valueWidth < 8 {
		valueWidth = 8
	}

	rows := make([]string, 0, len(m.wave.groups)+2)
	for i, group := range m.wave.groups {
		value := ""
		if group.index >= 0 && group.index < len(group.options) {
			value = group.options[group.index].name
		}

		// the value of the selected row is framed with arrows: they show
		// where the left and the right mouse buttons are
		valueCell := padRight(value, valueWidth)
		if i == m.wave.index && valueWidth >= 6 {
			valueCell = "‹ " + padRight(value, valueWidth-4) + " ›"
		}

		line := padRight(group.title, labelWidth) + "  " + valueCell
		rowStyle := lipgloss.NewStyle().Padding(0, 1)
		if i == m.wave.index {
			rowStyle = rowStyle.
				Foreground(style.InactiveTextColor).
				Background(style.AccentColor)
		} else {
			rowStyle = rowStyle.Foreground(style.NormalTextColor)
		}
		rows = append(rows, rowStyle.Render(line))
	}

	// the apply row is below the groups
	rows = append(rows, "")
	applyStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(style.NormalTextColor)
	if m.wave.index == len(m.wave.groups) {
		applyStyle = applyStyle.
			Foreground(style.InactiveTextColor).
			Background(style.AccentColor)
	}
	rows = append(rows, applyStyle.Render("Применить"))

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

// placeWaveSettings draws the window in the middle of the screen and
// remembers where every row landed.
func (m *Model) placeWaveSettings() string {
	view := m.waveView()
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
	m.wave.layout = waveLayout{
		x:          x,
		y:          y,
		width:      width,
		height:     height,
		rowX:       x + box.GetBorderLeftSize() + box.GetPaddingLeft() + 1,
		rowY:       y + box.GetBorderTopSize() + box.GetPaddingTop() + 2,
		rowWidth:   _WAVE_SETTINGS_WIDTH,
		labelWidth: m.wave.labelWidth(),
		rows:       len(m.wave.groups),
		valid:      true,
	}

	return placed
}

// openWaveSettings shows the window with the settings of the main wave.
func (m *Model) openWaveSettings() tea.Cmd {
	if m.isWaveActive {
		return nil
	}

	m.isWaveActive = true
	m.wave = &waveSettingsModel{loading: true}

	go func() {
		m.Send(waveStationsMsg{stations: m.loadStations()})
	}()

	return tea.ClearScreen
}

// waveControl handles the keys and the mouse while the window is open.
func (m *Model) waveControl(message tea.Msg) (tea.Cmd, bool) {
	if !m.isWaveActive || m.wave == nil {
		return nil, false
	}

	switch msg := message.(type) {
	case tea.KeyMsg:
		keypress := msg.String()

		switch keypress {
		case "esc":
			m.isWaveActive = false
			m.wave = nil
			return tea.ClearScreen, true
		case "up":
			m.wave.up()
			return nil, true
		case "down":
			m.wave.down()
			return nil, true
		case "left":
			m.wave.change(-1)
			return nil, true
		case "right":
			m.wave.change(1)
			return nil, true
		case "enter", " ":
			if m.wave.index == len(m.wave.groups) {
				m.isWaveActive = false
				m.wave = nil
				return tea.Batch(tea.ClearScreen, m.applyWaveSettings()), true
			}
			return nil, true
		}

		// the window swallows every other key
		return nil, true

	case tea.MouseMsg:
		return m.waveMouse(msg), true
	}

	return nil, false
}

// waveMouse handles the mouse while the window is open.
func (m *Model) waveMouse(msg tea.MouseMsg) tea.Cmd {
	if m.wave == nil {
		return nil
	}
	layout := m.wave.layout

	switch msg.Action {
	case tea.MouseActionMotion:
		if msg.Button != tea.MouseButtonNone {
			break
		}
		if row, ok := layout.rowAt(msg.X, msg.Y); ok && row != m.wave.index {
			m.wave.index = row
		}

	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonMiddle {
			break
		}

		row, ok := layout.rowAt(msg.X, msg.Y)
		if !ok {
			// a click outside of the window closes it
			m.isWaveActive = false
			m.wave = nil
			return tea.ClearScreen
		}

		m.wave.index = row
		if row == len(m.wave.groups) {
			m.isWaveActive = false
			m.wave = nil
			return tea.Batch(tea.ClearScreen, m.applyWaveSettings())
		}

		// the halves of the value work the way the arrows point
		m.wave.change(layout.valueDir(msg.X))
	}

	return nil
}

// waveSelection returns the options that are chosen in the window.
func (m *Model) waveSelection() (activity, diversity, mood, language string) {
	if m.wave == nil {
		return
	}

	return m.wave.option("activity").value,
		m.wave.option("diversity").value,
		m.wave.option("mood").value,
		m.wave.option("language").value
}

// waveSeeds returns the seeds of the main wave: the radio itself and the
// chosen settings.
func (m *Model) waveSeeds() []string {
	wave := config.Current.Wave
	if wave == nil {
		return []string{api.MyWaveId.String()}
	}

	return waveSeeds(wave.Activity, wave.Diversity, wave.MoodEnergy, wave.Language)
}

// waveSeedsWith returns the seeds of the main wave for the given settings.
func waveSeeds(activity, diversity, mood, language string) []string {
	seeds := []string{api.MyWaveId.String()}

	if activity != "" {
		seeds = append(seeds, "activity:"+activity)
	}
	if mood != "" {
		seeds = append(seeds, "settingMoodEnergy:"+mood)
	}
	if diversity != "" {
		seeds = append(seeds, "settingDiversity:"+diversity)
	}
	if language != "" {
		seeds = append(seeds, "settingLanguage:"+language)
	}

	return seeds
}

// applyWaveSettings saves the chosen settings and starts a new session of the
// main wave with them.
func (m *Model) applyWaveSettings() tea.Cmd {
	activity, diversity, mood, language := m.waveSelection()
	m.storeWaveSettings(activity, diversity, mood, language)
	m.saveConfig()

	seeds := waveSeeds(activity, diversity, mood, language)

	m.waveGeneration++
	generation := m.waveGeneration

	if m.client == nil {
		return nil
	}

	m.isLoading = true
	go func() {
		defer m.Send(LOADING_DONE)

		session, err := m.client.RotorNewSessionWithSeeds(seeds)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to start the wave: %s", err)
			m.tracker.ShowError("wave session")
			return
		}

		m.Send(waveSessionMsg{generation: generation, session: session})
	}()

	return m.spinner.Tick
}

// storeWaveSettings puts the chosen options into the config.
func (m *Model) storeWaveSettings(activity, diversity, mood, language string) {
	if config.Current.Wave == nil {
		config.Current.Wave = &config.Wave{}
	}

	config.Current.Wave.Activity = activity
	config.Current.Wave.Diversity = diversity
	config.Current.Wave.MoodEnergy = mood
	config.Current.Wave.Language = language

	summary := m.waveDescription()
	if summary == "обычная" {
		summary = ""
	}
	config.Current.Wave.Summary = summary
}

// applyWaveSession replaces the session of the main wave with the new one:
// the track that is playing now stays, the next ones come from the new wave.
func (m *Model) applyWaveSession(msg waveSessionMsg) {
	if msg.generation != m.waveGeneration {
		// a newer session is already on its way
		return
	}
	if len(msg.session.Sequence) == 0 {
		return
	}

	for i, item := range m.playlists.Items() {
		if item.Kind != playlist.MYWAVE {
			continue
		}

		item.SessionId = msg.session.RadioSessionId
		item.SessionBatch = msg.session.BatchId
		if len(msg.session.AcceptedSeeds) > 0 {
			item.StationId = msg.session.AcceptedSeeds[0]
		} else {
			item.StationId = msg.session.Id
		}

		tracks := make([]api.Track, 0, len(msg.session.Sequence)+1)
		for _, entry := range msg.session.Sequence {
			tracks = append(tracks, entry.Track)
		}

		item.Tracks = tracks
		item.SelectedTrack = 0
		item.CurrentTrack = 0
		m.playlists.SetItem(i, item)

		if m.currentPlaylistIndex == i {
			// the list is refreshed, so the new settings are shown right
			// next to the name of the wave
			m.displayPlaylist(item)

			// the wave starts right away: the new settings have to be heard
			m.playSelectedPlaylist(0)
		}

		m.showToast("Моя волна: " + m.waveDescription())
		break
	}
}

// waveDescription returns a short description of the chosen settings.
func (m *Model) waveDescription() string {
	if m.wave == nil {
		return waveSettingsSummary()
	}

	parts := make([]string, 0, 4)
	for _, key := range []string{"activity", "diversity", "mood", "language"} {
		option := m.wave.option(key)
		if option.name == "" {
			continue
		}
		parts = append(parts, option.name)
	}
	if len(parts) == 0 {
		return "обычная"
	}

	return strings.Join(parts, " • ")
}

// waveSettingsSummary describes the settings of the wave that are in the
// config. The names are the ones the user sees in the window.
func waveSettingsSummary() string {
	wave := config.Current.Wave
	if wave == nil {
		return ""
	}
	if wave.Summary != "" {
		return wave.Summary
	}

	values := []string{wave.Activity, wave.Diversity, wave.MoodEnergy, wave.Language}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		parts = append(parts, waveOptionName(value))
	}

	return strings.Join(parts, " • ")
}

// waveOptionName returns the name of a setting of the wave. The stations list
// knows every option with the name the user sees.
func waveOptionName(value string) string {
	for _, station := range cachedStations() {
		if station.Station.Id.Type == "activity" && station.Station.Id.Tag == value {
			return station.Station.Name
		}

		for _, option := range station.Station.Restrictions2.Diversity.PossibleValues {
			if option.Value == value {
				return option.Name
			}
		}
		for _, option := range station.Station.Restrictions2.MoodEnergy.PossibleValues {
			if option.Value == value {
				return option.Name
			}
		}
		for _, option := range station.Station.Restrictions2.Language.PossibleValues {
			if option.Value == value {
				return option.Name
			}
		}
	}

	return value
}
