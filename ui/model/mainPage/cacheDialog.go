package mainpage

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/cache"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
	"github.com/phantom-lancer/yatui/ui/style"
)

// cacheListMsg carries the tracks found in the cache directory. Reading the
// ID3 tags is done off the UI thread.
type cacheListMsg struct {
	tracks []api.Track
	err    error
}

type cacheModel struct {
	index   int
	offset  int
	tracks  []api.Track
	total   int64
	loading bool
	err     string
}

func loadCacheList() tea.Cmd {
	return func() tea.Msg {
		tracks, err := cache.ListTracks()
		return cacheListMsg{tracks: tracks, err: err}
	}
}

// toggleCache opens or closes the cache manager.
func (m *Model) toggleCache() tea.Cmd {
	if m.isCacheActive {
		m.isCacheActive = false
		m.cache = nil
		return nil
	}

	m.cache = &cacheModel{loading: true}
	m.isCacheActive = true
	return loadCacheList()
}

// applyCacheList fills the dialog with the loaded tracks.
func (m *Model) applyCacheList(msg cacheListMsg) {
	if m.cache == nil {
		return
	}

	m.cache.loading = false
	if msg.err != nil {
		m.cache.err = msg.err.Error()
		return
	}

	m.cache.tracks = msg.tracks
	m.cache.total = 0
	for _, track := range msg.tracks {
		m.cache.total += int64(track.FileSize)
	}
	m.cache.ensureVisible(m.height-14, true)
}

// cacheControl handles keys while the cache manager is open.
func (m *Model) cacheControl(message tea.Msg) (tea.Cmd, bool) {
	if !m.isCacheActive || m.cache == nil {
		return nil, false
	}

	msg, ok := message.(tea.KeyMsg)
	if !ok {
		return nil, false
	}

	controls := config.Current.Controls
	keypress := msg.String()

	switch {
	case keypress == "esc", controls.Cache.Contains(keypress):
		m.isCacheActive = false
		m.cache = nil
		return nil, true
	case keypress == "up":
		m.cache.move(-1)
		m.cache.ensureVisible(m.height-14, true)
	case keypress == "down":
		m.cache.move(1)
		m.cache.ensureVisible(m.height-14, true)
	case keypress == "enter":
		cmd := m.playCachedTrack(m.cache.index)
		m.isCacheActive = false
		m.cache = nil
		return cmd, true
	case keypress == "delete", keypress == "backspace":
		return m.deleteCachedTrack(m.cache.index), true
	}

	return nil, true
}

// playCachedTrack switches to the local playlist and plays the given track.
func (m *Model) playCachedTrack(index int) tea.Cmd {
	if m.cache == nil || index < 0 || index >= len(m.cache.tracks) {
		return nil
	}

	track := m.cache.tracks[index]
	localPlaylist, listIndex := m.playlists.GetFirst(playlist.LOCAL)
	if localPlaylist == nil {
		// no local playlist (should not happen, but the file can still be
		// played straight from the cache)
		m.playTrack(&track)
		return nil
	}

	playIndex := -1
	for i := range localPlaylist.Tracks {
		if localPlaylist.Tracks[i].Id == track.Id {
			playIndex = i
			break
		}
	}
	if playIndex < 0 {
		localPlaylist.AddTrack(&track)
		m.playlists.SetItem(listIndex, localPlaylist)
		playIndex = len(localPlaylist.Tracks) - 1
	}

	m.currentPlaylistIndex = listIndex
	m.playQueue = localPlaylist.Tracks
	m.playQueueTrackIndex = playIndex
	m.playTrack(&m.playQueue[playIndex])
	return nil
}

// deleteCachedTrack removes the file and updates every list that shows it.
func (m *Model) deleteCachedTrack(index int) tea.Cmd {
	if m.cache == nil || index < 0 || index >= len(m.cache.tracks) {
		return nil
	}

	track := m.cache.tracks[index]
	cmd := m.removeCache(&track)

	// removeCache reports a problem with a toast and leaves the file alone
	if !cache.Exists(track.Id) {
		m.cache.total -= int64(track.FileSize)
		m.cache.tracks = append(m.cache.tracks[:index], m.cache.tracks[index+1:]...)
		if m.cache.index >= len(m.cache.tracks) {
			m.cache.index = len(m.cache.tracks) - 1
		}
		if m.cache.index < 0 {
			m.cache.index = 0
		}
		m.cache.ensureVisible(m.height-14, true)
	}

	return cmd
}

func (c *cacheModel) move(delta int) {
	c.index += delta
	if c.index < 0 {
		c.index = 0
	}
	if c.index >= len(c.tracks) {
		c.index = len(c.tracks) - 1
	}
	if c.index < 0 {
		c.index = 0
	}
}

func (c *cacheModel) ensureVisible(height int, clampIndex bool) {
	if clampIndex {
		if c.index >= len(c.tracks) {
			c.index = len(c.tracks) - 1
		}
		if c.index < 0 {
			c.index = 0
		}
	}
	if height <= 0 {
		c.offset = 0
		return
	}
	if c.index < c.offset {
		c.offset = c.index
	} else if c.index >= c.offset+height {
		c.offset = c.index - height + 1
	}
	maxOffset := len(c.tracks) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if c.offset > maxOffset {
		c.offset = maxOffset
	}
}

// cacheView renders the cache manager dialog.
func (m *Model) cacheView() string {
	const dialogWidth = 64

	rows := make([]string, 0, 8)
	rows = append(rows, style.TrackVersionStyle.Render(
		fmt.Sprintf("Файлов: %d, размер: %s", len(m.cache.tracks), humanSize(m.cache.total)),
	))
	rows = append(rows, "")

	if m.cache.loading {
		rows = append(rows, style.TrackVersionStyle.Render("Читаю кэш…"))
	} else if m.cache.err != "" {
		rows = append(rows, style.ErrorTextStyle.Render(m.cache.err))
	} else if len(m.cache.tracks) == 0 {
		rows = append(rows, style.TrackVersionStyle.Render("Кэш пуст. Текущий трек кэшируется клавишей S."))
	}

	height := m.height - 16
	if height < 3 {
		height = 3
	}
	if height > len(m.cache.tracks) && len(m.cache.tracks) > 0 {
		height = len(m.cache.tracks)
	}
	m.cache.ensureVisible(height, true)

	visibleEnd := m.cache.offset + height
	if visibleEnd > len(m.cache.tracks) {
		visibleEnd = len(m.cache.tracks)
	}

	for i := m.cache.offset; i < visibleEnd; i++ {
		track := &m.cache.tracks[i]
		line := fmt.Sprintf("%d. %s", i+1, trackLabel(track))
		size := humanSize(int64(track.FileSize))

		// the size is right aligned, the title is cut first
		line = padRight(line, dialogWidth-2-len(size)-2) + "  " + size
		line = padRight(line, dialogWidth-2)

		rowStyle := lipgloss.NewStyle().Padding(0, 1)
		if i == m.cache.index {
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
		style.DialogTitleStyle.Render("Кэш"),
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		box,
		style.DialogHelpStyle.Render("↑/↓ — выбрать, enter — играть, delete — удалить, esc — закрыть"),
	)
}

// humanSize formats a byte count for the dialog.
func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d Б", size)
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cБ", float64(size)/float64(div), []rune("КМГТ")[exp])
}
