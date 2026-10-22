package mainpage

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
	"github.com/phantom-lancer/yatui/ui/components/search"
	"github.com/phantom-lancer/yatui/ui/components/tracker"
	"github.com/phantom-lancer/yatui/ui/model"
)

// _TRACK_RESTART_THRESHOLD is the playback position after which the
// previous track hotkey restarts the current track instead of
// switching to the previous one.
const _TRACK_RESTART_THRESHOLD = 4 * time.Second

// isPlayerHotkey reports whether the key controls the player itself.
// Such keys are handled globally, regardless of the focused element.
func isPlayerHotkey(controls *config.Controls, keypress string) bool {
	return controls.PlayerRewindBackward.Contains(keypress) ||
		controls.PlayerStop.Contains(keypress) ||
		controls.PlayerRewindForward.Contains(keypress)
}

// playerHotkey executes a player hotkey from any page state.
func (m *Model) playerHotkey(keypress string) tea.Cmd {
	controls := config.Current.Controls

	switch {
	case controls.PlayerRewindBackward.Contains(keypress):
		return m.restartOrPreviousTrack()

	case controls.PlayerStop.Contains(keypress):
		return m.togglePlayback()

	case controls.PlayerRewindForward.Contains(keypress):
		return model.Cmd(tracker.NEXT)
	}

	return nil
}

// restartOrPreviousTrack rewinds the current track to its beginning or,
// if it has just started, switches to the previous track.
func (m *Model) restartOrPreviousTrack() tea.Cmd {
	if m.tracker.IsStoped() {
		return nil
	}

	if m.tracker.Position() > _TRACK_RESTART_THRESHOLD {
		return tea.Batch(
			m.tracker.SeekToStart(),
			model.Cmd(tracker.REWIND),
		)
	}

	return model.Cmd(tracker.PREV)
}

// Rect is a rectangular area of the screen.
type Rect struct {
	X, Y, Width, Height int
}

func (r Rect) IsEmpty() bool {
	return r.Width <= 0 || r.Height <= 0
}

func (r Rect) Contains(x, y int) bool {
	if r.IsEmpty() {
		return false
	}
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// fraction returns the horizontal position of x inside the rect (0..1).
func (r Rect) fraction(x int) float64 {
	fraction := float64(x-r.X) / float64(r.Width)
	if fraction < 0 {
		return 0
	}
	if fraction > 1 {
		return 1
	}
	return fraction
}

// Offset returns the rect moved to the given screen coordinates.
func (r Rect) Offset(x, y int) Rect {
	r.X += x
	r.Y += y
	return r
}

// toRect converts a player region to the screen region.
func toRect(region tracker.Rect) Rect {
	return Rect{X: region.X, Y: region.Y, Width: region.Width, Height: region.Height}
}

// mouseLayout holds screen coordinates of the interactive parts
// of the last rendered view.
type mouseLayout struct {
	playlist  Rect
	tracklist Rect
	settings  Rect
	prev      Rect
	play      Rect
	next      Rect
	eq        Rect
	progress  Rect
	volume    Rect
	volumeBar Rect
	like      Rect
	dislike   Rect
}

// updateMouseLayout places component views on the screen.
func (m *Model) updateMouseLayout(playlistView, tracklistView, trackerView, midPanel string) {
	m.mouse = buildMouseLayout(
		m.tracklist.Hidden,
		m.tracker.Hidden,
		playlistView, tracklistView, trackerView, midPanel,
		m.tracker.Regions(),
	)

	// the settings button is a part of the sidebar
	m.mouse.settings = placeSidebarFooter(m.mouse.playlist, toScreenRect(m.playlists.FooterRect()))
}

// toScreenRect converts an area of the sidebar view to the screen coordinates.
func toScreenRect(r playlist.Rect) Rect {
	return Rect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
}

// placeSidebarFooter maps the footer of the sidebar to the screen.
func placeSidebarFooter(playlist, footer Rect) Rect {
	if footer.IsEmpty() {
		return Rect{}
	}

	return Rect{
		X:      playlist.X + footer.X,
		Y:      playlist.Y + footer.Y,
		Width:  footer.Width,
		Height: footer.Height,
	}
}

// buildMouseLayout converts component views to their screen coordinates.
func buildMouseLayout(tracklistHidden, trackerHidden bool, playlistView, tracklistView, trackerView, midPanel string, regions *tracker.Regions) mouseLayout {
	var layout mouseLayout

	playlistWidth := lipgloss.Width(playlistView)
	playlistHeight := lipgloss.Height(playlistView)

	midX := playlistWidth
	midY := 0
	midHeight := lipgloss.Height(midPanel)
	if playlistHeight > midHeight {
		// panels are aligned to the bottom
		midY = playlistHeight - midHeight
	}

	layout.playlist = Rect{
		X:      0,
		Y:      midY,
		Width:  playlistWidth,
		Height: playlistHeight,
	}
	if midHeight > playlistHeight {
		// the sidebar is aligned to the bottom as well
		layout.playlist.Y = midHeight - playlistHeight
	}

	if !tracklistHidden {
		layout.tracklist = Rect{
			X:      midX,
			Y:      midY,
			Width:  lipgloss.Width(tracklistView),
			Height: lipgloss.Height(tracklistView),
		}
	}

	if !trackerHidden && regions != nil {
		// player occupies the bottom of the middle panel
		trackerY := midY + midHeight - lipgloss.Height(trackerView)
		layout.prev = toRect(regions.PrevButton.Offset(midX, trackerY))
		layout.play = toRect(regions.PlayButton.Offset(midX, trackerY))
		layout.next = toRect(regions.NextButton.Offset(midX, trackerY))
		layout.eq = toRect(regions.EQ.Offset(midX, trackerY))
		layout.progress = toRect(regions.Progress.Offset(midX, trackerY))
		layout.volume = toRect(regions.Volume.Offset(midX, trackerY))
		layout.volumeBar = toRect(regions.VolumeBar.Offset(midX, trackerY))
		layout.like = toRect(regions.Like.Offset(midX, trackerY))
		layout.dislike = toRect(regions.Dislike.Offset(midX, trackerY))
	}

	return layout
}

// handleMouse processes mouse events over the main page.
func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if m.isLoading || m.isSearchActive || m.isAddPlaylistActive || m.isRenamePlaylistActive {
		return nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.mouseScroll(msg.X, msg.Y, true)
	case tea.MouseButtonWheelDown:
		return m.mouseScroll(msg.X, msg.Y, false)
	}

	if msg.Action == tea.MouseActionMotion {
		// the motion with the button held is the drag of a track
		if msg.Button == tea.MouseButtonNone || msg.Button == tea.MouseButtonLeft {
			return m.mouseMotion(msg.X, msg.Y)
		}
	}

	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		return m.mousePress(msg.X, msg.Y)
	}

	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonRight {
		return m.mouseRightClick(msg.X, msg.Y)
	}

	if msg.Action == tea.MouseActionRelease {
		if msg.Button == tea.MouseButtonLeft || msg.Button == tea.MouseButtonNone {
			return m.mouseRelease(msg.X, msg.Y)
		}
	}

	return nil
}

// mouseMotion moves the selection to the element under the cursor, so the
// pointer works like the cursor keys.
func (m *Model) mouseMotion(x, y int) tea.Cmd {
	hover := m.mouse.settings.Contains(x, y)
	if hover != m.settingsHover {
		m.settingsHover = hover
	}

	if hover {
		return nil
	}

	// tracks list
	if m.mouse.tracklist.Contains(x, y) {
		index := m.tracklist.ItemAt(x-m.mouse.tracklist.X, y-m.mouse.tracklist.Y)
		if m.tracklist.Dragging() {
			// the track follows the mouse
			m.tracklist.DragTo(index)
			return nil
		}
		if index >= 0 && index != m.tracklist.Index() {
			m.tracklist.Select(index)
			// the tracks of the queue are not a part of the playlist: they
			// do not change the selection
			if playlistIndex := m.playlistTrackIndex(index); playlistIndex >= 0 {
				return m.selectTrack(playlistIndex)
			}
		}
		return nil
	}

	// playlists: the pointer only marks the item under it, the panel is
	// switched by a click or by the keyboard
	if m.mouse.playlist.Contains(x, y) {
		index := m.playlists.ItemAt(x-m.mouse.playlist.X, y-m.mouse.playlist.Y)
		if index != m.playlists.Hovered() {
			m.playlists.SetHovered(index)
		}
	} else if m.playlists.Hovered() != -1 {
		m.playlists.SetHovered(-1)
	}

	return nil
}

//
// left button: the press takes hold of a track, the release plays it
//

func (m *Model) mousePress(x, y int) tea.Cmd {
	// settings button
	if m.mouse.settings.Contains(x, y) {
		m.toggleSettings()
		return tea.ClearScreen
	}

	switch {
	case m.mouse.prev.Contains(x, y):
		return model.Cmd(tracker.PREV)

	case m.mouse.play.Contains(x, y):
		return m.togglePlayback()

	case m.mouse.next.Contains(x, y):
		return model.Cmd(tracker.NEXT)

	case m.mouse.eq.Contains(x, y):
		return m.openEqualizer()

	case m.mouse.progress.Contains(x, y):
		return tea.Batch(
			m.tracker.SeekTo(m.mouse.progress.fraction(x)),
			model.Cmd(tracker.REWIND),
		)

	case m.mouse.volumeBar.Contains(x, y):
		m.tracker.SetVolumeTo(m.mouse.volumeBar.fraction(x))
		return model.Cmd(tracker.VOLUME)

	case m.mouse.dislike.Contains(x, y):
		return m.dislikePlayingTrack()

	case m.mouse.like.Contains(x, y):
		return m.likePlayingTrack()
	}

	// tracks list: the track under the cursor is taken hold of, it is
	// played or moved when the button is released
	if m.mouse.tracklist.Contains(x, y) {
		rowX := x - m.mouse.tracklist.X
		rowY := y - m.mouse.tracklist.Y

		if index := m.tracklist.ItemAt(rowX, rowY); index >= 0 {
			// the tracks of the queue are not a part of the playlist: they
			// are moved in the queue window
			items := m.tracklist.Items()
			if index < len(items) && !items[index].IsNext {
				m.tracklist.BeginDrag(index)
			}
		}
		return nil
	}

	// playlists
	if m.mouse.playlist.Contains(x, y) {
		index := m.playlists.ItemAt(x-m.mouse.playlist.X, y-m.mouse.playlist.Y)
		if m.playlists.ClickItem(index) {
			m.followSelectedPlaylist()
		}
	}

	return nil
}

// mouseRelease finishes the drag of a track. A click without a movement plays
// the track.
func (m *Model) mouseRelease(x, y int) tea.Cmd {
	if !m.mouse.tracklist.Contains(x, y) {
		// the button was released outside of the list: the drag is over
		m.tracklist.EndDrag()
		return nil
	}

	rowX := x - m.mouse.tracklist.X
	rowY := y - m.mouse.tracklist.Y
	index := m.tracklist.ItemAt(rowX, rowY)

	if m.tracklist.EndDrag() {
		// the tracks were moved: the new order belongs to the playlist
		return m.applyTracklistOrder()
	}

	// the button was pressed and released in the same place: it is a click
	if index < 0 {
		return nil
	}

	if likeIndex := m.tracklist.LikeAt(rowX, rowY); likeIndex >= 0 {
		return m.likeTrackAt(likeIndex)
	}

	playlistIndex := m.playlistTrackIndex(index)
	if playlistIndex < 0 {
		// a track of the queue: it is played right away
		m.playQueueEntry(m.queueEntries(), index)
		return nil
	}

	return m.playTrackAt(playlistIndex)
}

// applyTracklistOrder saves the new order of the tracks in the playlist.
func (m *Model) applyTracklistOrder() tea.Cmd {
	selected := m.playlists.SelectedItem()
	if selected == nil {
		return nil
	}

	items := m.tracklist.Items()
	tracks := make([]api.Track, 0, len(items))
	for _, item := range items {
		// the tracks of the queue are not a part of the playlist
		if item.IsNext || item.Track == nil {
			continue
		}
		tracks = append(tracks, *item.Track)
	}

	selected.Tracks = tracks
	m.playlists.SetItem(m.playlists.Index(), selected)

	// the queue follows the new order, the track that plays now keeps its
	// place in it
	if m.currentPlaylistIndex == m.playlists.Index() {
		m.playQueue = tracks
		if current := m.tracker.CurrentTrack(); current != nil {
			for i, track := range tracks {
				if track.Id == current.Id {
					m.playQueueTrackIndex = i
					break
				}
			}
		}
	}

	return nil
}

func (m *Model) togglePlayback() tea.Cmd {
	if m.tracker.IsStoped() {
		// nothing is loaded yet, start the selected track
		return m.playTrackAt(m.playlistTrackIndex(m.tracklist.Index()))
	}
	if m.tracker.IsPlaying() {
		return model.Cmd(tracker.PAUSE)
	}
	return model.Cmd(tracker.PLAY)
}

// playTrackAt selects and plays the track at the given index of the tracks list.
func (m *Model) playTrackAt(index int) tea.Cmd {
	if index < 0 || len(m.playlists.Items()) == 0 {
		return nil
	}
	if !m.playlists.SelectedItem().Active {
		return nil
	}

	m.tracklist.Select(m.tracklistIndex(index))
	cmd := m.selectTrack(index)

	if m.albumListActive() {
		return tea.Batch(cmd, m.openAlbum(index))
	}
	m.playSelectedPlaylist(index)
	return cmd
}

// playRadioByTrack starts a radio session based on the given track.
func (m *Model) playRadioByTrack(track *api.Track) tea.Cmd {
	if m.client == nil || track == nil {
		return nil
	}

	session, err := m.client.RotorNewSession(api.StationId{Type: "track", Tag: track.Id})
	if err != nil {
		m.tracker.ShowError("radio session")
		return nil
	}

	st := &playlist.Item{
		Name:         "radio: " + track.Title,
		Kind:         playlist.MYWAVE,
		Active:       true,
		Subitem:      false,
		Rotor:        true,
		SessionId:    session.RadioSessionId,
		SessionBatch: session.BatchId,
	}
	if len(session.AcceptedSeeds) > 0 {
		st.StationId = session.AcceptedSeeds[0]
	} else {
		st.StationId = session.Id
	}
	if len(session.Sequence) > 0 {
		st.Tracks = []api.Track{session.Sequence[0].Track}
	} else {
		m.tracker.ShowError("radio session")
		return nil
	}

	m.playlists.InsertItem(len(m.playlists.Items()), st)
	m.playlists.Select(len(m.playlists.Items()) - 1)
	m.followSelectedPlaylist()

	m.playSelectedPlaylist(0)
	return nil
}

// likeTrackAt likes the track at the given index of the tracks list.
func (m *Model) likeTrackAt(index int) tea.Cmd {
	if m.currentPlaylistIndex < 0 || len(m.playlists.Items()) == 0 {
		return nil
	}

	selectedPlaylist := m.playlists.SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 {
		return nil
	}

	items := m.tracklist.Items()
	if index < 0 || index >= len(items) || items[index].Track == nil {
		return nil
	}

	return tea.Batch(
		m.selectTrack(m.playlistTrackIndex(index)),
		m.likeTrack(items[index].Track, selectedPlaylist),
	)
}

// mouseRightClick opens a context menu for the clicked track or playlist.
func (m *Model) mouseRightClick(x, y int) tea.Cmd {
	// tracks list
	if m.mouse.tracklist.Contains(x, y) {
		rowX := x - m.mouse.tracklist.X
		rowY := y - m.mouse.tracklist.Y
		index := m.tracklist.ItemAt(rowX, rowY)
		if index >= 0 {
			items := m.tracklist.Items()
			if index < len(items) && items[index].Track != nil {
				m.openContextMenu(index, x, y)
			}
			return nil
		}
	}

	// playlists sidebar
	if m.mouse.playlist.Contains(x, y) {
		index := m.playlists.ItemAt(x-m.mouse.playlist.X, y-m.mouse.playlist.Y)
		if index >= 0 {
			pl := m.playlists.Items()
			if index < len(pl) && pl[index].Active {
				m.openPlaylistMenu(pl[index], x, y)
			}
		}
	}

	return nil
}

// openPlaylistMenu builds the context menu of a sidebar playlist.
func (m *Model) openPlaylistMenu(pl *playlist.Item, x, y int) {
	items := make([]contextMenuItem, 0, 3)

	if pl.Kind == playlist.MYWAVE {
		items = append(items, contextMenuItem{
			label: "Настроить Мою волну",
			action: func(m *Model) tea.Cmd {
				return m.openWaveSettings()
			},
		})
	}

	if pl.Rotor {
		items = append(items, contextMenuItem{
			label: "Change mood",
			action: func(m *Model) tea.Cmd {
				return m.openStationPicker(x, y)
			},
		})
	}

	if pl.Kind == playlist.LIKES || pl.Kind == playlist.LOCAL {
		items = append(items, contextMenuItem{
			label: "Shuffle",
			action: func(m *Model) tea.Cmd {
				return m.shufflePlaylist(pl)
			},
		})
	}

	if len(items) == 0 {
		return
	}

	m.contextMenu = &contextMenu{
		x:     x,
		y:     y,
		items: items,
	}
}

// openContextMenu builds and shows the context menu for a track.
func (m *Model) openContextMenu(trackIdx, x, y int) {
	items := m.tracklist.Items()
	if trackIdx < 0 || trackIdx >= len(items) {
		return
	}
	track := items[trackIdx].Track
	if track == nil {
		return
	}

	plIdx := m.playlists.Index()
	selectedPlaylist := m.playlists.SelectedItem()

	menuItems := []contextMenuItem{
		{label: "Play", action: func(m *Model) tea.Cmd { return m.playTrackAt(trackIdx) }},
		{label: "Play next", action: func(m *Model) tea.Cmd {
			m.addNext(track)
			return nil
		}},
		{label: "Add to queue", action: func(m *Model) tea.Cmd {
			m.addToQueue(track)
			return nil
		}},
		{label: "Start radio", action: func(m *Model) tea.Cmd { return m.playRadioByTrack(track) }},
		{label: "Add to playlist", action: func(m *Model) tea.Cmd {
			m.searchDialog.Title = "Add " + track.Title + " to"
			m.searchDialog.Action = "add"
			m.isAddPlaylistActive = true
			m.Send(search.UPDATE_SUGGESTIONS)
			return nil
		}},
		{label: "Like", action: func(m *Model) tea.Cmd { return m.likeTrackAt(trackIdx) }},
		{label: "Dislike", action: func(m *Model) tea.Cmd { return m.dislikeTrackAt(trackIdx) }},
		{label: "Share", action: func(m *Model) tea.Cmd {
			link := api.ShareTrackLink(track)
			if link != "" {
				m.clipboard.CopyText(link)
				m.showToast("Ссылка скопирована")
			}
			return nil
		}},
		{label: "Show artist", action: func(m *Model) tea.Cmd {
			return m.showArtist(track)
		}},
	}

	if selectedPlaylist != nil && selectedPlaylist.Kind >= playlist.USER && selectedPlaylist.Kind != playlist.MYWAVE {
		menuItems = append(menuItems, contextMenuItem{
			label: "Remove from playlist",
			action: func(m *Model) tea.Cmd {
				return m.removeFromPlaylist(selectedPlaylist, trackIdx)
			},
		})
	}

	m.contextMenu = &contextMenu{
		x:        x,
		y:        y,
		items:    menuItems,
		trackIdx: trackIdx,
		plIdx:    plIdx,
	}
}

// selectTrack moves the track selection to the given index.
func (m *Model) selectTrack(index int) tea.Cmd {
	if len(m.playlists.Items()) == 0 {
		return nil
	}
	selectedPlaylist := m.playlists.SelectedItem()
	selectedPlaylist.SelectedTrack = index
	return m.playlists.SetItem(m.playlists.Index(), selectedPlaylist)
}

//
// wheel scroll
//

func (m *Model) mouseScroll(x, y int, up bool) tea.Cmd {
	if m.mouse.volume.Contains(x, y) {
		// wheel up raises the volume, wheel down lowers it
		step := config.Current.VolumeStep
		if !up {
			step = -step
		}
		m.tracker.NudgeVolume(step)
		return model.Cmd(tracker.VOLUME)
	}

	if m.mouse.progress.Contains(x, y) {
		// wheel up moves forward, wheel down moves back
		amount := time.Duration(config.Current.RewindDuration) * time.Second
		if !up {
			amount = -amount
		}
		return tea.Batch(
			m.tracker.Rewind(amount),
			model.Cmd(tracker.REWIND),
		)
	}

	if m.mouse.tracklist.Contains(x, y) {
		if len(m.playlists.Items()) == 0 {
			return nil
		}

		var cmd tea.Cmd
		if up {
			cmd = m.tracklist.CursorUp()
		} else {
			cmd = m.tracklist.CursorDown()
		}
		if cmd == nil {
			return nil
		}
		return tea.Batch(cmd, m.selectTrack(m.playlistTrackIndex(m.tracklist.Index())))
	}

	if m.mouse.playlist.Contains(x, y) {
		if len(m.playlists.Items()) == 0 {
			return nil
		}
		if up {
			m.playlists.CursorUp()
		} else {
			m.playlists.CursorDown()
		}
		m.followSelectedPlaylist()
		return nil
	}

	return nil
}
