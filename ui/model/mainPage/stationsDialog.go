package mainpage

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/ui/style"
)

// _STATIONS_WIDTH is the width of the window with the stations.
const _STATIONS_WIDTH = 62

// stationsLoadedMsg carries the stations the window is built of.
type stationsLoadedMsg struct {
	stations []api.StationDesc
}

// stationLine is one row of the window: either the name of a group of
// stations or a station itself.
type stationLine struct {
	text string
	// item is the index of the station in the list, -1 for a group name.
	item int
}

// stationsDialogModel is the window with the list of the stations.
type stationsDialogModel struct {
	input textinput.Model
	// all is every station, items are the ones that match the search.
	all     []api.StationDesc
	items   []api.StationDesc
	lines   []stationLine
	index   int
	offset  int
	loading bool
}

// openStations shows the window with the list of the stations.
func (m *Model) openStations() tea.Cmd {
	if m.isStationsActive {
		return nil
	}

	input := textinput.New()
	input.Placeholder = "поиск по станциям"
	input.Prompt = "> "
	input.Width = _STATIONS_WIDTH - 6
	input.Focus()

	m.isStationsActive = true
	m.stations = &stationsDialogModel{input: input, loading: true}

	go func() {
		m.Send(stationsLoadedMsg{stations: m.loadStations()})
	}()

	return tea.ClearScreen
}

// setStations fills the window with the stations and applies the filter.
func (s *stationsDialogModel) setStations(stations []api.StationDesc) {
	s.loading = false
	s.all = stations
	s.applyFilter()
}

// applyFilter leaves only the stations whose name matches the query.
func (s *stationsDialogModel) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(s.input.Value()))

	items := s.all
	if query != "" {
		items = make([]api.StationDesc, 0, 32)
		for _, station := range s.all {
			if strings.Contains(strings.ToLower(station.Station.Name), query) {
				items = append(items, station)
			}
		}
	}

	s.items = items
	s.lines = s.buildLines(items)

	// the cursor starts at the first station, not at the name of a group
	s.index = 0
	for i, line := range s.lines {
		if line.item >= 0 {
			s.index = i
			break
		}
	}
	s.offset = 0
}

// buildLines lays the stations out as the rows of the window: the name of a
// group is shown when the group changes.
func (s *stationsDialogModel) buildLines(items []api.StationDesc) []stationLine {
	lines := make([]stationLine, 0, len(items)+10)

	lastType := ""
	for i, station := range items {
		if station.Station.Id.Type != lastType {
			lastType = station.Station.Id.Type
			lines = append(lines, stationLine{text: stationTypeName(lastType), item: -1})
		}
		lines = append(lines, stationLine{text: station.Station.Name, item: i})
	}

	return lines
}

// stationTypeName is the name of a group of stations.
func stationTypeName(stationType string) string {
	switch stationType {
	case "genre":
		return "Жанры"
	case "micro-genre":
		return "Поджанры"
	case "mood":
		return "Настроения"
	case "activity":
		return "Занятия"
	case "epoch":
		return "Эпохи"
	case "editorial":
		return "Редакционные"
	case "personal":
		return "Личные"
	case "author":
		return "Авторы"
	case "tempo":
		return "Темп"
	case "local-language":
		return "Язык"
	default:
		return "Миксы"
	}
}

// up moves the selection to the previous station.
func (s *stationsDialogModel) up() {
	for i := s.index - 1; i >= 0; i-- {
		if s.lines[i].item >= 0 {
			s.index = i
			return
		}
	}
}

// down moves the selection to the next station.
func (s *stationsDialogModel) down() {
	for i := s.index + 1; i < len(s.lines); i++ {
		if s.lines[i].item >= 0 {
			s.index = i
			return
		}
	}
}

// selected returns the station under the cursor.
func (s *stationsDialogModel) selected() (api.StationDesc, bool) {
	if s.index < 0 || s.index >= len(s.lines) {
		return api.StationDesc{}, false
	}
	item := s.lines[s.index].item
	if item < 0 || item >= len(s.items) {
		return api.StationDesc{}, false
	}
	return s.items[item], true
}

// ensureVisible keeps the selected row inside the window.
func (s *stationsDialogModel) ensureVisible(height int) {
	if s.index < 0 {
		s.index = 0
	}
	if height <= 0 {
		s.offset = 0
		return
	}
	if s.index < s.offset {
		s.offset = s.index
	} else if s.index >= s.offset+height {
		s.offset = s.index - height + 1
	}
	maxOffset := len(s.lines) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if s.offset > maxOffset {
		s.offset = maxOffset
	}
	if s.offset < 0 {
		s.offset = 0
	}
}

// stationsControl handles the keys and the mouse while the window is open.
func (m *Model) stationsControl(message tea.Msg) (tea.Cmd, bool) {
	if !m.isStationsActive || m.stations == nil {
		return nil, false
	}

	switch msg := message.(type) {
	case tea.KeyMsg:
		keypress := msg.String()

		switch keypress {
		case "esc":
			m.isStationsActive = false
			m.stations = nil
			return tea.ClearScreen, true
		case "up":
			m.stations.up()
			return nil, true
		case "down":
			m.stations.down()
			return nil, true
		case "enter":
			station, ok := m.stations.selected()
			if !ok {
				return nil, true
			}
			m.isStationsActive = false
			m.stations = nil
			return tea.Batch(tea.ClearScreen, m.startStation(station)), true
		}

		// everything else goes into the search field
		var cmd tea.Cmd
		m.stations.input, cmd = m.stations.input.Update(msg)
		m.stations.applyFilter()
		return cmd, true

	case tea.MouseMsg:
		return m.stationsMouse(msg), true
	}

	return nil, false
}

// stationsMouse handles the mouse while the window is open.
func (m *Model) stationsMouse(msg tea.MouseMsg) tea.Cmd {
	if m.stations == nil {
		return nil
	}

	layout := m.stationsLayout

	switch msg.Action {
	case tea.MouseActionMotion:
		if msg.Button != tea.MouseButtonNone {
			break
		}
		if row, ok := layout.rowAt(msg.X, msg.Y); ok && row != m.stations.index {
			m.stations.index = row
		}

	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonMiddle {
			break
		}
		row, ok := layout.rowAt(msg.X, msg.Y)
		if !ok {
			m.isStationsActive = false
			m.stations = nil
			return tea.ClearScreen
		}
		m.stations.index = row
		station, ok := m.stations.selected()
		if !ok {
			return nil
		}
		m.isStationsActive = false
		m.stations = nil
		return tea.Batch(tea.ClearScreen, m.startStation(station))
	}

	return nil
}

// stationsLayout is where the rows of the window ended up on the screen.
type stationsLayout struct {
	x, y          int
	width, height int
	rowX, rowY    int
	rowWidth      int
	rows          int
	valid         bool
}

// rowAt returns the index of the row under the given screen coordinates.
func (l stationsLayout) rowAt(x, y int) (int, bool) {
	if !l.valid || x < l.x || x >= l.x+l.width || y < l.rowY {
		return 0, false
	}
	if row := y - l.rowY; row < l.rows {
		return row, true
	}
	return 0, false
}

// stationsHeight is the number of the rows the window shows.
func (m *Model) stationsHeight() int {
	height := m.height - 12
	if height < 5 {
		height = 5
	}
	if m.stations != nil && len(m.stations.lines) > 0 && height > len(m.stations.lines) {
		height = len(m.stations.lines)
	}
	return height
}

// stationsView renders the window with the stations.
func (m *Model) stationsView() string {
	title := style.DialogTitleStyle.Render("Станции")

	if m.stations == nil || m.stations.loading {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			style.DialogBoxStyle.Render(lipgloss.JoinVertical(
				lipgloss.Left,
				title,
				style.DialogHelpStyle.Render("Загрузка..."),
			)),
			style.DialogHelpStyle.Render("esc — выход"),
		)
	}

	height := m.stationsHeight()
	m.stations.ensureVisible(height)

	rows := make([]string, 0, height+4)
	rows = append(rows, m.stations.input.View())
	rows = append(rows, "")

	visibleEnd := m.stations.offset + height
	if visibleEnd > len(m.stations.lines) {
		visibleEnd = len(m.stations.lines)
	}

	if len(m.stations.lines) == 0 {
		rows = append(rows, style.TrackVersionStyle.Render("Ничего не найдено"))
	}

	for i := m.stations.offset; i < visibleEnd; i++ {
		line := m.stations.lines[i]
		text := "  " + line.text
		if line.item < 0 {
			text = line.text
		}

		rowStyle := lipgloss.NewStyle().Padding(0, 1)
		if i == m.stations.index {
			rowStyle = rowStyle.
				Foreground(style.InactiveTextColor).
				Background(style.AccentColor)
		} else if line.item < 0 {
			rowStyle = rowStyle.Foreground(style.AccentColor)
		} else {
			rowStyle = rowStyle.Foreground(style.NormalTextColor)
		}

		rows = append(rows, rowStyle.Render(padRight(text, _STATIONS_WIDTH-2)))
	}

	if m.stations.offset > 0 {
		rows = append(rows, style.TrackVersionStyle.Render("… выше"))
	}
	if visibleEnd < len(m.stations.lines) {
		rows = append(rows, style.TrackVersionStyle.Render("… ниже"))
	}

	box := style.DialogBoxStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	))

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		style.DialogHelpStyle.Render("↑/↓ — выбор, enter — слушать, esc — выход"),
	)

	// the window is placed in the middle of the screen: the rows are counted
	// from its top
	width := lipgloss.Width(view)
	total := lipgloss.Height(view)
	x := (m.width - width) / 2
	y := (m.height - total) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	frame := style.DialogBoxStyle
	m.stationsLayout = stationsLayout{
		x:     x,
		y:     y,
		width: width,
		rowX:  x + frame.GetBorderLeftSize() + frame.GetPaddingLeft(),
		// the title with its margin, the search field and the empty line
		rowY:  y + frame.GetBorderTopSize() + frame.GetPaddingTop() + 4,
		rows:  height,
		valid: true,
	}

	return view
}
