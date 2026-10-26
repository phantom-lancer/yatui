package mainpage

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/ui/components/search"
	"github.com/phantom-lancer/yatui/ui/components/tracklist"
	"github.com/phantom-lancer/yatui/ui/style"
)

// _TRACK_SEARCH_WIDTH is the width of the quick track search window.
const _TRACK_SEARCH_WIDTH = 70

// _TRACK_SEARCH_ROWS is how many results are shown at once.
const _TRACK_SEARCH_ROWS = 12

// trackSearchModel is the quick search over the tracks of the open list.
//
// It never changes the tracklist itself: the indices point into the snapshot
// taken when the window was opened, so playback mapping stays intact.
type trackSearchModel struct {
	query   string
	items   []tracklist.Item
	results []int
	index   int
	// layout is the last rendered position, used by the mouse.
	x, y, width, height int
	firstRow            int
	valid               bool
}

// openGlobalSearch opens the Yandex search dialog: it searches the whole
// catalogue, not only the open list.
func (m *Model) openGlobalSearch() tea.Cmd {
	if m.searchDialog == nil {
		return nil
	}
	m.trackSearch = nil
	m.searchDialog.Title = "Поиск в Яндексе"
	m.searchDialog.Action = "search"
	m.isSearchActive = true
	m.Send(search.UPDATE_SUGGESTIONS)
	return tea.ClearScreen
}

// openTrackSearch shows the quick search over the current list.
func (m *Model) openTrackSearch() tea.Cmd {
	if m.tracklist == nil {
		return nil
	}
	ts := &trackSearchModel{items: m.tracklist.Items()}
	ts.refresh()
	m.trackSearch = ts
	return tea.ClearScreen
}

// closeTrackSearch hides the quick search.
func (m *Model) closeTrackSearch() tea.Cmd {
	m.trackSearch = nil
	return tea.ClearScreen
}

// refresh recomputes the results for the current query. Matching is case
// insensitive and checks the title and the artists.
func (ts *trackSearchModel) refresh() {
	ts.results = ts.results[:0]
	needle := strings.ToLower(strings.TrimSpace(ts.query))

	for i, item := range ts.items {
		if item.Track == nil {
			continue
		}
		if needle == "" {
			ts.results = append(ts.results, i)
			continue
		}
		haystack := strings.ToLower(item.Track.Title + " " + item.Artists)
		if strings.Contains(haystack, needle) {
			ts.results = append(ts.results, i)
		}
	}

	if len(ts.results) == 0 {
		ts.index = 0
		return
	}
	if ts.index >= len(ts.results) {
		ts.index = len(ts.results) - 1
	}
	if ts.index < 0 {
		ts.index = 0
	}
}

// up moves the selection up.
func (ts *trackSearchModel) up() {
	if ts.index > 0 {
		ts.index--
	}
}

// down moves the selection down.
func (ts *trackSearchModel) down() {
	if ts.index < len(ts.results)-1 {
		ts.index++
	}
}

// selected returns the index of the selected track in the snapshot, or -1.
func (ts *trackSearchModel) selected() int {
	if ts.index < 0 || ts.index >= len(ts.results) {
		return -1
	}
	return ts.results[ts.index]
}

// visibleRange returns the first and the last result that fit the window.
func (ts *trackSearchModel) visibleRange() (int, int) {
	if len(ts.results) <= _TRACK_SEARCH_ROWS {
		return 0, len(ts.results)
	}
	first := ts.index - _TRACK_SEARCH_ROWS + 1
	if first < 0 {
		first = 0
	}
	last := first + _TRACK_SEARCH_ROWS
	if last > len(ts.results) {
		last = len(ts.results)
		first = last - _TRACK_SEARCH_ROWS
	}
	return first, last
}

// trackSearchView renders the quick search window.
func (m *Model) trackSearchView() string {
	ts := m.trackSearch
	if ts == nil {
		return ""
	}

	inner := _TRACK_SEARCH_WIDTH - 6
	if inner < 20 {
		inner = 20
	}

	title := style.DialogTitleStyle.Render("Поиск треков")

	cursor := "█"
	input := "› " + ts.query + cursor
	if lipgloss.Width(input) > inner {
		runes := []rune(ts.query)
		trim := inner - 3
		if trim < 0 {
			trim = 0
		}
		if len(runes) > trim {
			runes = runes[len(runes)-trim:]
		}
		input = "› " + string(runes) + cursor
	}

	rows := make([]string, 0, _TRACK_SEARCH_ROWS+4)
	rows = append(rows, input)
	rows = append(rows, "")

	first, last := ts.visibleRange()
	if len(ts.results) == 0 {
		rows = append(rows, style.DialogHelpStyle.Render("ничего не найдено"))
	}
	for pos := first; pos < last; pos++ {
		item := ts.items[ts.results[pos]]
		if item.Track == nil {
			continue
		}

		line := item.Track.Title
		if item.Artists != "" {
			line += " — " + item.Artists
		}
		line = padRightCells(line, inner)
		if lipgloss.Width(line) > inner {
			line = truncateCells(line, inner)
		}

		rowStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(style.NormalTextColor)
		if pos == ts.index {
			rowStyle = rowStyle.Foreground(style.InactiveTextColor).Background(style.AccentColor)
		}
		rows = append(rows, rowStyle.Render(line))
	}

	help := style.DialogHelpStyle.Render("↑/↓ — выбор, enter — играть, tab — поиск в Яндексе, esc — выход")

	box := style.DialogBoxStyle.Width(_TRACK_SEARCH_WIDTH).Render(lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	))

	return lipgloss.JoinVertical(lipgloss.Left, box, help)
}

// placeTrackSearch draws the window in the middle of the screen and remembers
// where the result rows landed.
func (m *Model) placeTrackSearch() string {
	view := m.trackSearchView()
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

	// border(1) + padding(1) + title(1) + title margin(1) + input(1) + blank(1)
	firstRow := y + 6

	m.trackSearch.x = x
	m.trackSearch.y = y
	m.trackSearch.width = width
	m.trackSearch.height = height
	m.trackSearch.firstRow = firstRow
	m.trackSearch.valid = true

	return placed
}

// trackSearchControl handles the keys and the mouse while the window is open.
func (m *Model) trackSearchControl(message tea.Msg) (tea.Cmd, bool) {
	ts := m.trackSearch
	if ts == nil {
		return nil, false
	}

	switch msg := message.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch key {
		case "esc":
			return m.closeTrackSearch(), true
		case "enter":
			return m.playTrackSearchResult(), true
		case "tab":
			return m.openGlobalSearch(), true
		case "up", "ctrl+p":
			ts.up()
			return nil, true
		case "down", "ctrl+n":
			ts.down()
			return nil, true
		case "backspace":
			runes := []rune(ts.query)
			if len(runes) > 0 {
				ts.query = string(runes[:len(runes)-1])
			}
			ts.refresh()
			return nil, true
		case "space":
			ts.query += " "
			ts.refresh()
			return nil, true
		}

		// printable characters extend the query
		if len(key) == 1 || (len([]rune(key)) == 1 && []rune(key)[0] >= 0x20) {
			ts.query += key
			ts.refresh()
			return nil, true
		}

		// any other key is swallowed
		return nil, true

	case tea.MouseMsg:
		return m.trackSearchMouse(msg), true
	}

	return nil, false
}

// trackSearchMouse handles the mouse while the quick search is open.
func (m *Model) trackSearchMouse(msg tea.MouseMsg) tea.Cmd {
	ts := m.trackSearch
	if ts == nil || !ts.valid {
		return nil
	}

	switch msg.Action {
	case tea.MouseActionMotion:
		if row, ok := ts.rowAt(msg.X, msg.Y); ok {
			ts.index = row
		}
		return nil

	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonMiddle {
			return nil
		}
		if msg.X < ts.x || msg.X >= ts.x+ts.width || msg.Y < ts.y || msg.Y >= ts.y+ts.height {
			return m.closeTrackSearch()
		}
		if row, ok := ts.rowAt(msg.X, msg.Y); ok {
			ts.index = row
			return m.playTrackSearchResult()
		}
	}

	return nil
}

// rowAt converts a screen row to the index in the result list.
func (ts *trackSearchModel) rowAt(x, y int) (int, bool) {
	if !ts.valid || x < ts.x || x >= ts.x+ts.width || y < ts.firstRow {
		return 0, false
	}
	first, _ := ts.visibleRange()
	row := first + (y - ts.firstRow)
	if row < first || row >= len(ts.results) {
		return 0, false
	}
	return row, true
}

// playTrackSearchResult plays the selected search result and closes the
// window. The dashboard indices are mapped back to the real list.
func (m *Model) playTrackSearchResult() tea.Cmd {
	ts := m.trackSearch
	if ts == nil {
		return nil
	}
	original := ts.selected()
	if original < 0 || original >= len(ts.items) {
		return m.closeTrackSearch()
	}

	cmd := m.closeTrackSearch()

	item := ts.items[original]
	if item.IsNext {
		m.playQueueEntry(m.queueEntries(), original)
		return cmd
	}

	playlistIndex := m.playlistTrackIndex(original)
	if playlistIndex < 0 {
		return cmd
	}
	return tea.Batch(cmd, m.playTrackAt(playlistIndex))
}

// padRight pads a string with spaces up to the given cell width.
func padRightCells(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// truncateCells cuts a string to the given cell width.
func truncateCells(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:0]
	}
	runes := []rune(s)
	// a simple cut is enough for the list: ANSI codes are not used here
	if len(runes) > width-1 {
		runes = runes[:width-1]
	}
	return string(runes) + "…"
}
