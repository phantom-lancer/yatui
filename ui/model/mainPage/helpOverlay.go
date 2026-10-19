package mainpage

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dece2183/yamusic-tui/config"
	"github.com/dece2183/yamusic-tui/ui/style"
)

const (
	// _HELP_KEY_WIDTH is the width of the column with the names of the keys.
	_HELP_KEY_WIDTH = 14
	// _HELP_TEXT_WIDTH is the width of the column with the descriptions.
	_HELP_TEXT_WIDTH = 22
	// _HELP_CHROME is the number of lines the window spends on its frame:
	// the border, the padding, the title and the hint under it.
	_HELP_CHROME = 8
)

// helpItem is a single key binding of the help window.
type helpItem struct {
	key         *config.Key
	description string
}

// helpSection is a group of the key bindings.
type helpSection struct {
	title string
	items []helpItem
}

// helpSections returns all the key bindings of the application, grouped into
// sections.
func helpSections() []helpSection {
	controls := config.Current.Controls

	return []helpSection{
		{
			title: "Общее",
			items: []helpItem{
				{controls.Quit, "выход"},
				{controls.Apply, "применить"},
				{controls.Cancel, "отмена"},
				{controls.Reload, "перезагрузить"},
				{controls.ShowAllKeys, "это окно"},
				{controls.Theme, "сменить тему"},
				{controls.Settings, "настройки"},
				{controls.Queue, "очередь"},
				{controls.Cache, "кэш"},
			},
		},
		{
			title: "Плейлисты",
			items: []helpItem{
				{controls.PlaylistsUp, "вверх"},
				{controls.PlaylistsDown, "вниз"},
				{controls.PlaylistsRename, "переименовать"},
				{controls.PlaylistsHide, "скрыть панель"},
			},
		},
		{
			title: "Треки",
			items: []helpItem{
				{controls.CursorUp, "вверх"},
				{controls.CursorDown, "вниз"},
				{controls.TracksNextPage, "страница вперёд"},
				{controls.TracksPrevPage, "страница назад"},
				{controls.TracksLike, "нравится"},
				{controls.TracksArtist, "исполнитель"},
				{controls.TracksAddToPlaylist, "в плейлист"},
				{controls.TracksRemoveFromPlaylist, "убрать из плейлиста"},
				{controls.TracksShare, "поделиться"},
				{controls.TracksShuffle, "перемешать"},
				{controls.TracksSearch, "поиск"},
				{controls.TracksBack, "назад"},
				{controls.TracksHide, "скрыть панель"},
			},
		},
		{
			title: "Плеер",
			items: []helpItem{
				{controls.PlayerPause, "пауза"},
				{controls.PlayerStop, "стоп"},
				{controls.PlayerNext, "следующий трек"},
				{controls.PlayerPrevious, "предыдущий трек"},
				{controls.PlayerRewindForward, "перемотка вперёд"},
				{controls.PlayerRewindBackward, "перемотка назад"},
				{controls.PlayerLike, "нравится"},
				{controls.PlayerCache, "кэшировать"},
				{controls.PlayerVolUp, "громче"},
				{controls.PlayerVolDown, "тише"},
				{controls.PlayerToggleLyrics, "слова"},
				{controls.PlayerHide, "скрыть плеер"},
			},
		},
	}
}

// keyDisplayName returns the name of a key as the user configured it.
func keyDisplayName(k *config.Key) string {
	if k == nil {
		return ""
	}

	binding := key.NewBinding(k.Binding(), k.Help(""))
	return binding.Help().Key
}

// helpRows lays the sections out as rows: a title of a section and the
// bindings of it. The rows of every section are returned separately, so the
// sections are never split between the columns.
func helpRows() [][]string {
	sections := helpSections()
	groups := make([][]string, 0, len(sections))

	for _, section := range sections {
		rows := make([]string, 0, len(section.items)+2)
		rows = append(rows, style.AccentTextStyle.Render(section.title))

		for _, item := range section.items {
			name := padRight(keyDisplayName(item.key), _HELP_KEY_WIDTH)
			rows = append(rows, lipgloss.NewStyle().Foreground(style.NormalTextColor).Render(name)+
				lipgloss.NewStyle().Foreground(style.InactiveTextColor).Render(padRight(item.description, _HELP_TEXT_WIDTH)))
		}

		groups = append(groups, rows)
	}

	return groups
}

// helpView renders the window with the key bindings.
func (m *Model) helpView() string {
	groups := helpRows()

	// only the rows that fit into the window are shown, the rest is reached
	// with the arrow keys
	visible := m.height - _HELP_CHROME
	if visible < 4 {
		visible = 4
	}

	// the first column takes the sections that fit into the window, the rest
	// of them goes to the second one
	first := make([]string, 0)
	second := make([]string, 0)
	overflow := false
	for _, group := range groups {
		if !overflow && (len(first) == 0 || len(first)+len(group) <= visible) {
			first = append(first, group...)
			continue
		}
		overflow = true
		second = append(second, group...)
	}
	columns := [][]string{first, second}

	// the second column is the one that does not fit, it scrolls
	offset := m.helpOffset
	if offset > len(second)-visible {
		offset = len(second) - visible
	}
	if offset < 0 {
		offset = 0
	}

	views := make([]string, 0, len(columns))
	for i, rows := range columns {
		start := 0
		if i > 0 {
			start = offset
		}
		end := start + visible
		if end > len(rows) {
			end = len(rows)
		}

		column := lipgloss.JoinVertical(lipgloss.Left, rows[start:end]...)
		views = append(views, lipgloss.NewStyle().Width(_HELP_KEY_WIDTH+_HELP_TEXT_WIDTH).Render(column))
	}

	box := style.DialogBoxStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		style.DialogTitleStyle.Render("Горячие клавиши"),
		lipgloss.JoinHorizontal(lipgloss.Top, views...),
	))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		style.DialogHelpStyle.Render("↑/↓ — прокрутка, esc — выход"),
	)
}

// placeHelp draws the help window in the middle of the screen.
func (m *Model) placeHelp() string {
	view := m.helpView()
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

	m.helpRect = Rect{X: x, Y: y, Width: width, Height: height}
	return placed
}

// toggleHelp opens or closes the window with the key bindings.
func (m *Model) toggleHelp() {
	m.isHelpActive = !m.isHelpActive
	m.helpOffset = 0
}

// helpControl handles the keys and the mouse while the help window is open.
func (m *Model) helpControl(message tea.Msg) (tea.Cmd, bool) {
	if !m.isHelpActive {
		return nil, false
	}

	controls := config.Current.Controls

	switch msg := message.(type) {
	case tea.KeyMsg:
		keypress := msg.String()

		switch {
		case controls.Cancel.Contains(keypress), controls.ShowAllKeys.Contains(keypress):
			m.isHelpActive = false
			return tea.ClearScreen, true
		case controls.CursorUp.Contains(keypress):
			m.helpOffset--
			return nil, true
		case controls.CursorDown.Contains(keypress):
			m.helpOffset++
			return nil, true
		}

		// the window swallows every other key
		return nil, true

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.helpOffset--
			return nil, true
		case tea.MouseButtonWheelDown:
			m.helpOffset++
			return nil, true
		}

		if msg.Action == tea.MouseActionPress && (msg.Button == tea.MouseButtonLeft || msg.Button == tea.MouseButtonMiddle) {
			// a click outside of the window closes it
			if !m.helpRect.Contains(msg.X, msg.Y) {
				m.isHelpActive = false
				return tea.ClearScreen, true
			}
		}

		return nil, true
	}

	// the other messages (the playback ones) are not the business of the
	// window and have to reach the components
	return nil, false
}
