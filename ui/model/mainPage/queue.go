package mainpage

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/helpers"
	"github.com/phantom-lancer/yatui/ui/style"
)

// queueModel is the state of the queue dialog: the selection and the first
// visible row.
type queueModel struct {
	index  int
	offset int
}

// queueLayout is where the rows of the window ended up on the screen.
type queueLayout struct {
	x, y  int
	width int
	rowY  int
	rows  int
	valid bool
}

// queueEntry is one row of the queue. manual entries were put there by the
// user and can be removed; the rest is the upcoming part of the playlist.
type queueEntry struct {
	track  api.Track
	manual bool
}

// addNext puts a track right after the current one.
func (m *Model) addNext(track *api.Track) {
	if track == nil {
		return
	}
	m.manualQueue = append([]api.Track{*track}, m.manualQueue...)
	m.showToast("Далее: " + track.Title)
}

// addToQueue appends a track to the end of the manual queue.
func (m *Model) addToQueue(track *api.Track) {
	if track == nil {
		return
	}
	m.manualQueue = append(m.manualQueue, *track)
	m.showToast("В очередь: " + track.Title)
}

// queueEntries returns the manual queue followed by the upcoming tracks of
// the current playlist. Only the first entries are built: the dialog is a
// snapshot and a ten thousand track playlist must not be copied on every
// repaint.
func (m *Model) queueEntries() []queueEntry {
	const maxEntries = 300

	entries := make([]queueEntry, 0, len(m.manualQueue)+8)
	for _, track := range m.manualQueue {
		if len(entries) >= maxEntries {
			return entries
		}
		entries = append(entries, queueEntry{track: track, manual: true})
	}
	for i := m.playQueueTrackIndex + 1; i < len(m.playQueue) && len(entries) < maxEntries; i++ {
		entries = append(entries, queueEntry{track: m.playQueue[i]})
	}
	return entries
}

// playQueueIndex jumps to a track of the current playlist.
func (m *Model) playQueueIndex(index int) {
	if index < 0 || index >= len(m.playQueue) {
		return
	}

	m.playQueueTrackIndex = index
	if m.currentPlaylistIndex >= 0 && m.currentPlaylistIndex < len(m.playlists.Items()) {
		currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
		currentPlaylist.CurrentTrack = index
		m.playlists.SetItem(m.currentPlaylistIndex, currentPlaylist)
	}

	track := &m.playQueue[index]
	if !track.Available {
		m.showToast("Трек недоступен")
		return
	}
	m.playTrack(track)
}

// playQueueEntry plays the given queue row.
func (m *Model) playQueueEntry(entries []queueEntry, index int) {
	if index < 0 || index >= len(entries) {
		return
	}

	entry := entries[index]
	if entry.manual {
		// remove the manual entry from the queue and play it right away
		for i := range m.manualQueue {
			if m.manualQueue[i].Id == entry.track.Id {
				m.manualQueue = append(m.manualQueue[:i], m.manualQueue[i+1:]...)
				break
			}
		}
		track := entry.track
		m.playTrack(&track)
		return
	}

	// map the row back to the playlist queue
	manualCount := len(m.manualQueue)
	queueIndex := m.playQueueTrackIndex + 1 + (index - manualCount)
	m.playQueueIndex(queueIndex)
}

func (q *queueModel) up() {
	if q.index > 0 {
		q.index--
	}
}

func (q *queueModel) down(items int) {
	if q.index < items-1 {
		q.index++
	}
}

// ensureVisible keeps the selection inside the visible window.
func (q *queueModel) ensureVisible(items, height int) {
	if q.index < 0 {
		q.index = 0
	}
	if q.index >= items && items > 0 {
		q.index = items - 1
	}
	if height <= 0 {
		q.offset = 0
		return
	}
	if q.index < q.offset {
		q.offset = q.index
	} else if q.index >= q.offset+height {
		q.offset = q.index - height + 1
	}
	maxOffset := items - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if q.offset > maxOffset {
		q.offset = maxOffset
	}
}

// toggleQueue opens or closes the queue dialog.
func (m *Model) toggleQueue() {
	if m.isQueueActive {
		m.isQueueActive = false
		m.queue = nil
		return
	}
	m.queue = &queueModel{}
	m.isQueueActive = true
}

// queueControl handles keys while the queue dialog is open.
func (m *Model) queueControl(message tea.Msg) (tea.Cmd, bool) {
	if !m.isQueueActive || m.queue == nil {
		return nil, false
	}

	msg, ok := message.(tea.KeyMsg)
	if !ok {
		return nil, false
	}

	controls := config.Current.Controls
	entries := m.queueEntries()
	keypress := msg.String()

	switch {
	case keypress == "esc", controls.Queue.Contains(keypress):
		m.isQueueActive = false
		m.queue = nil
	case keypress == "up":
		m.queue.up()
	case keypress == "down":
		m.queue.down(len(entries))
	case keypress == "ctrl+up", keypress == "k", keypress == "K":
		m.moveManualQueueEntry(m.queue.index, -1)
	case keypress == "ctrl+down", keypress == "j", keypress == "J":
		m.moveManualQueueEntry(m.queue.index, 1)
	case keypress == "p", keypress == "P":
		m.moveManualQueueToTop(m.queue.index)
	case keypress == "enter":
		m.playQueueEntry(entries, m.queue.index)
		m.isQueueActive = false
		m.queue = nil
	case keypress == "delete", keypress == "backspace":
		if m.queue.index >= 0 && m.queue.index < len(entries) && entries[m.queue.index].manual {
			m.removeManualQueueEntry(m.queue.index)
		}
	case keypress == "c":
		m.manualQueue = nil
		m.queue.index = 0
		m.queue.offset = 0
		m.showToast("Очередь очищена")
	}

	return tea.ClearScreen, true
}

// moveManualQueueEntry moves the manual entry of the queue by the given
// number of places. Only the tracks the user added can be moved.
func (m *Model) moveManualQueueEntry(index, delta int) {
	if index < 0 || index >= len(m.manualQueue) {
		return
	}

	target := index + delta
	if target < 0 || target >= len(m.manualQueue) {
		return
	}

	m.manualQueue[index], m.manualQueue[target] = m.manualQueue[target], m.manualQueue[index]

	// the selection follows the moved track
	if m.queue != nil {
		m.queue.index = target
	}
}

// moveManualQueueToTop puts the manual entry of the queue at the first place:
// it will be played right after the current track.
func (m *Model) moveManualQueueToTop(index int) {
	if index <= 0 || index >= len(m.manualQueue) {
		return
	}

	track := m.manualQueue[index]
	m.manualQueue = append(m.manualQueue[:index], m.manualQueue[index+1:]...)
	m.manualQueue = append([]api.Track{track}, m.manualQueue...)

	if m.queue != nil {
		m.queue.index = 0
	}
}

func (m *Model) removeManualQueueEntry(index int) {
	if index < 0 || index >= len(m.manualQueue) {
		return
	}
	m.manualQueue = append(m.manualQueue[:index], m.manualQueue[index+1:]...)
	if m.queue.index >= len(m.manualQueue) {
		m.queue.index = len(m.manualQueue) - 1
	}
	if m.queue.index < 0 {
		m.queue.index = 0
	}
}

// rowAt returns the index of the row under the given screen coordinates.
func (l queueLayout) rowAt(x, y int) (int, bool) {
	if !l.valid || x < l.x || x >= l.x+l.width || y < l.rowY {
		return 0, false
	}
	if row := y - l.rowY; row < l.rows {
		return row, true
	}
	return 0, false
}

// queueMouse handles the mouse while the queue is open.
func (m *Model) queueMouse(msg tea.MouseMsg) tea.Cmd {
	if m.queue == nil {
		return nil
	}
	layout := m.queueLayout

	switch msg.Action {
	case tea.MouseActionMotion:
		if msg.Button != tea.MouseButtonNone {
			break
		}
		if row, ok := layout.rowAt(msg.X, msg.Y); ok && row != m.queue.index {
			m.queue.index = row
		}

	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonMiddle {
			break
		}
		row, ok := layout.rowAt(msg.X, msg.Y)
		if !ok {
			return nil
		}
		if row == m.queue.index {
			// the second click plays the track
			entries := m.queueEntries()
			m.playQueueEntry(entries, row)
			m.isQueueActive = false
			m.queue = nil
			return tea.ClearScreen
		}
		m.queue.index = row
	}

	return nil
}

// queueView renders the queue dialog.
func (m *Model) queueView() string {
	const dialogWidth = 64

	entries := m.queueEntries()
	height := m.height - 14
	if height < 3 {
		height = 3
	}
	if height > len(entries) && len(entries) > 0 {
		height = len(entries)
	}
	m.queue.ensureVisible(len(entries), height)

	rows := make([]string, 0, height+4)

	current := (*api.Track)(nil)
	if m.tracker != nil && !m.tracker.IsStoped() {
		current = m.tracker.CurrentTrack()
	}
	if current != nil {
		rows = append(rows, style.TrackVersionStyle.Render("Сейчас: "+trackLabel(current)))
	} else {
		rows = append(rows, style.TrackVersionStyle.Render("Сейчас: —"))
	}
	rows = append(rows, "")

	if len(entries) == 0 {
		rows = append(rows, style.TrackVersionStyle.Render("Очередь пуста"))
	}

	visibleEnd := m.queue.offset + height
	if visibleEnd > len(entries) {
		visibleEnd = len(entries)
	}

	for i := m.queue.offset; i < visibleEnd; i++ {
		entry := entries[i]

		marker := "  "
		if entry.manual {
			marker = "• "
		}
		line := fmt.Sprintf("%s%d. %s", marker, i+1, trackLabel(&entry.track))
		line = padRight(line, dialogWidth-2)

		rowStyle := lipgloss.NewStyle().Padding(0, 1)
		if i == m.queue.index {
			rowStyle = rowStyle.
				Foreground(style.InactiveTextColor).
				Background(style.AccentColor)
		} else {
			rowStyle = rowStyle.Foreground(style.NormalTextColor)
		}
		rows = append(rows, rowStyle.Render(line))
	}

	if len(entries) > 0 && m.queue.offset > 0 {
		rows = append(rows, style.TrackVersionStyle.Render(fmt.Sprintf("… ещё %d выше", m.queue.offset)))
	}
	if visibleEnd < len(entries) {
		rows = append(rows, style.TrackVersionStyle.Render(fmt.Sprintf("… ещё %d ниже", len(entries)-visibleEnd)))
	}

	box := style.DialogBoxStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		style.DialogTitleStyle.Render("Очередь"),
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	))

	// the window is placed in the middle of the screen: the rows are counted
	// from its top
	view := lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		style.DialogHelpStyle.Render("↑/↓ — выбор, ctrl+↑/↓ или k/j — переставить, p — в начало"),
		style.DialogHelpStyle.Render("enter — играть, delete — убрать, c — очистить, esc — закрыть"),
	)

	width := lipgloss.Width(view)
	height = lipgloss.Height(view)
	x := (m.width - width) / 2
	y := (m.height - height) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	// the border, the padding, the title with its margin and the two lines
	// above the rows
	const headerLines = 2 + 2 + 2
	m.queueLayout = queueLayout{
		x:     x,
		y:     y,
		width: width,
		rowY:  y + headerLines,
		rows:  len(entries),
		valid: true,
	}

	return view
}

// trackLabel is "Artist — Title" with a fallback to the plain title.
func trackLabel(track *api.Track) string {
	artists := helpers.ArtistList(track.Artists)
	if artists == "" {
		return track.Title
	}
	return artists + " — " + track.Title
}
