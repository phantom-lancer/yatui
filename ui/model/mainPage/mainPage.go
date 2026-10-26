package mainpage

import (
	"os"
	"path/filepath"
	"time"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/media/handler"
	"github.com/phantom-lancer/yatui/ui/components/input"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
	"github.com/phantom-lancer/yatui/ui/components/search"
	"github.com/phantom-lancer/yatui/ui/components/tracker"
	"github.com/phantom-lancer/yatui/ui/components/tracklist"
	"github.com/phantom-lancer/yatui/ui/model"
	"github.com/phantom-lancer/yatui/ui/model/hotkeys"
	"github.com/phantom-lancer/yatui/ui/style"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dece2183/go-clipboard"
)

type Model struct {
	program       *tea.Program
	client        *api.YaMusicClient
	clipboard     *clipboard.Clipboard
	mediaHandler  handler.MediaHandler
	width, height int

	spinner   spinner.Model
	playlists *playlist.Model
	tracklist *tracklist.Model
	tracker   *tracker.Model

	searchDialog           *search.Model
	inputDialog            *input.Model
	isLoading              bool
	isSearchActive         bool
	isAddPlaylistActive    bool
	isRenamePlaylistActive bool
	isPlaylistHideOverride bool

	playQueue            []api.Track
	playQueueTrackIndex  int
	currentPlaylistIndex int
	currentAlbumIndex    int

	likedTracksMap    map[string]bool
	dislikedTracksMap map[string]bool
	cachedTracksMap   map[string]bool

	mouse       mouseLayout
	hotkeys     *hotkeys.Tracker
	contextMenu *contextMenu

	toasts            []toast
	toastTimerPending bool
	// toastPlaylist is the name of the playlist the last toast reported
	toastPlaylist string

	settings         *settingsModel
	isSettingsActive bool
	settingsHover    bool

	// the window with all the key bindings
	isHelpActive bool
	helpOffset   int
	helpRect     Rect

	// the window with the settings of the main wave
	isWaveActive bool
	wave         *waveSettingsModel
	// waveGeneration grows with every new session of the wave: the sessions
	// that are already on their way are dropped when it changes
	waveGeneration int64

	// the window with the equalizer
	eq *eqModel

	// the window with the list of the stations
	isStationsActive bool
	stations         *stationsDialogModel
	// stationsLayout is where the rows of the window ended up on the screen.
	stationsLayout stationsLayout

	// failedTracks counts the broken tracks in a row: the playback gives up
	// when there are too many of them
	failedTracks int

	manualQueue   []api.Track
	queue         *queueModel
	isQueueActive bool
	// queueLayout is where the rows of the queue ended up on the screen.
	queueLayout queueLayout

	// renameLayout is where the rename dialog ended up on the screen.
	renameLayout renameLayout

	cache         *cacheModel
	isCacheActive bool

	// scrobbleGen invalidates the pending Last.fm scrobble
	scrobbleGen int64
}

// mainpage.Model constructor.
func New(mediaHandler handler.MediaHandler) *Model {
	m := &Model{}

	// all motion: the selection follows the cursor, the way "hover" works
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())
	m.program = p
	m.clipboard = clipboard.New()
	m.mediaHandler = mediaHandler
	m.hotkeys = hotkeys.NewTracker()
	m.hotkeys.Start(p)
	m.likedTracksMap = make(map[string]bool)
	m.dislikedTracksMap = make(map[string]bool)
	m.cachedTracksMap = make(map[string]bool)
	m.spinner = spinner.New(spinner.WithSpinner(spinner.Points))
	m.playlists = playlist.New(m.program, "YaMusic")
	m.tracklist = tracklist.New(m.program, &m.likedTracksMap, &m.dislikedTracksMap, &m.cachedTracksMap)
	m.tracker = tracker.New(m.program, &m.likedTracksMap, &m.dislikedTracksMap)
	m.searchDialog = search.New()
	m.inputDialog = input.New()

	return m
}

//
// model.Model interface implementation
//

func (m *Model) Run() error {
	go m.mediaHandle()
	_, err := m.program.Run()
	m.tracker.Stop()
	return err
}

func (m *Model) Send(msg tea.Msg) {
	if m.program == nil {
		return
	}
	go m.program.Send(msg)
}

//
// tea.Model interface implementation
//

func (m *Model) Init() tea.Cmd {
	m.isLoading = true
	go m.initialLoad()
	return tea.Batch(tea.SetWindowTitle("yatui"), m.spinner.Tick)
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// the notifications are pruned before anything else: no dialog and no
	// menu may swallow the timer, or the messages would stay on the screen
	if _, ok := message.(toastExpiredMsg); ok {
		m.toastTimerPending = false
		m.expireToasts()
		if len(m.toasts) > 0 {
			// the messages were refreshed while the timer was running
			m.scheduleToastExpiry()
		}
		return m, nil
	}

	// the help window captures the keys while it is open
	if cmd, handled := m.helpControl(message); handled {
		return m, cmd
	}

	// the wave settings window captures the keys while it is open
	if cmd, handled := m.waveControl(message); handled {
		return m, cmd
	}

	// the equalizer window captures the keys while it is open
	if cmd, handled := m.eqControl(message); handled {
		return m, cmd
	}

	// the stations window captures the keys while it is open
	if cmd, handled := m.stationsControl(message); handled {
		return m, cmd
	}

	// settings dialog captures the keys while it is open
	if cmd, handled := m.settingsControl(message); handled {
		return m, cmd
	}

	// queue dialog captures the keys while it is open
	if cmd, handled := m.queueControl(message); handled {
		return m, cmd
	}

	// cache manager captures the keys while it is open
	if cmd, handled := m.cacheControl(message); handled {
		return m, cmd
	}

	// clicks must not reach the components behind an open dialog
	if mouseMsg, isMouse := message.(tea.MouseMsg); isMouse {
		if m.isSettingsActive {
			return m, m.settingsMouse(mouseMsg)
		}
		if m.isQueueActive {
			return m, m.queueMouse(mouseMsg)
		}
		if m.isSearchActive || m.isAddPlaylistActive {
			return m, m.searchMouse(mouseMsg)
		}
		if m.isCacheActive {
			return m, m.cacheMouse(mouseMsg)
		}
		if m.isRenamePlaylistActive {
			return m, m.renameMouse(mouseMsg)
		}
		if m.eq != nil {
			return m, nil
		}
	}

	// async Last.fm auth results
	if m.handleLastfmMsg(message) {
		return m, nil
	}

	// context menu has priority over everything.
	// tea.ClearScreen is deliberately not used here: it would repaint the
	// whole screen on every mouse motion event and make the app flicker.
	if m.contextMenu != nil {
		switch msg := message.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc", "ctrl+c":
				m.contextMenu = nil
				return m, nil
			case "up":
				m.contextMenu.up()
				return m, nil
			case "down":
				m.contextMenu.down()
				return m, nil
			case "enter":
				cmd := m.contextMenu.selectItem(m)
				m.contextMenu = nil
				return m, cmd
			}
			return m, nil

		case tea.MouseMsg:
			// hovering highlights the item under the cursor
			if msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonNone {
				m.contextMenu.hover(msg.X, msg.Y)
				return m, nil
			}

			if msg.Action != tea.MouseActionPress {
				return m, nil
			}

			switch msg.Button {
			case tea.MouseButtonLeft, tea.MouseButtonMiddle:
				if !m.contextMenu.contains(msg.X, msg.Y) {
					m.contextMenu = nil
					return m, nil
				}
				idx := m.contextMenu.itemAt(msg.X, msg.Y)
				if idx < 0 {
					return m, nil
				}
				cmd := m.contextMenu.selectIndex(m, idx)
				m.contextMenu = nil
				return m, cmd

			case tea.MouseButtonRight:
				m.contextMenu = nil
				return m, nil
			}
		}
		return m, nil
	}

	switch msg := message.(type) {
	case LoadingMsg:
		m.isLoading = false
		return m, model.Cmd(playlist.CURSOR_UP)

	case cacheListMsg:
		m.applyCacheList(msg)
		return m, nil

	case waveStationsMsg:
		if m.wave != nil {
			m.wave.setStations(msg.stations)
			m.wave.loading = false
		}
		return m, nil

	case waveSessionMsg:
		m.applyWaveSession(msg)
		return m, nil

	case stationsLoadedMsg:
		if m.stations != nil {
			m.stations.setStations(msg.stations)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, tea.ClearScreen

	// mouse control update
	case tea.MouseMsg:
		cmd = m.handleMouse(msg)
		cmds = append(cmds, cmd)

	// function keys detected by the keyboard watcher
	case hotkeys.HotkeyMsg:
		if config.Current.Controls.Settings.Contains(msg.String()) {
			m.toggleSettings()
			cmds = append(cmds, tea.ClearScreen)
			break
		}
		if config.Current.Controls.WaveSettings.Contains(msg.String()) {
			cmds = append(cmds, m.openWaveSettings())
			break
		}
		if config.Current.Controls.Equalizer.Contains(msg.String()) {
			cmds = append(cmds, m.openEqualizer())
			break
		}
		cmds = append(cmds, m.playerHotkey(msg.String()))

	case tea.KeyMsg:
		m.hotkeys.Observe(msg)
		controls := config.Current.Controls
		keypress := msg.String()

		switch {
		case controls.Quit.Contains(keypress):
			return m, tea.Quit
		// player hotkeys are handled on any page state
		case isPlayerHotkey(controls, keypress):
			cmds = append(cmds, m.playerHotkey(keypress))
		case m.isSearchActive || m.isAddPlaylistActive:
			m.searchDialog, cmd = m.searchDialog.Update(message)
			cmds = append(cmds, cmd)
		case m.isRenamePlaylistActive:
			m.inputDialog, cmd = m.inputDialog.Update(message)
			cmds = append(cmds, cmd)
		case controls.Theme.Contains(keypress):
			m.cycleTheme()
			cmds = append(cmds, tea.ClearScreen)
		case controls.ShowAllKeys.Contains(keypress):
			m.toggleHelp()
			cmds = append(cmds, tea.ClearScreen)
		case controls.Settings.Contains(keypress):
			m.toggleSettings()
			cmds = append(cmds, tea.ClearScreen)
		case controls.WaveSettings.Contains(keypress):
			cmds = append(cmds, m.openWaveSettings())
		case controls.Equalizer.Contains(keypress):
			cmds = append(cmds, m.openEqualizer())
		case controls.Queue.Contains(keypress):
			m.toggleQueue()
			cmds = append(cmds, tea.ClearScreen)
		case controls.Cache.Contains(keypress):
			cmds = append(cmds, m.toggleCache())
			cmds = append(cmds, tea.ClearScreen)
		case controls.Reload.Contains(keypress):
			m.isLoading = true
			cmd = m.playlists.Reset()
			cmds = append(cmds, cmd)
			cmds = append(cmds, m.spinner.Tick)
			go m.initialLoad()
		default:
			if m.isLoading {
				m.spinner, cmd = m.spinner.Update(message)
				cmds = append(cmds, cmd)
			} else {
				m.playlists, cmd = m.playlists.Update(message)
				cmds = append(cmds, cmd)
				m.tracklist, cmd = m.tracklist.Update(message)
				cmds = append(cmds, cmd)
				m.tracker, cmd = m.tracker.Update(message)
				cmds = append(cmds, cmd)
			}
		}

	// playlist control update
	case playlist.Control:
		switch msg {
		case playlist.CURSOR_UP, playlist.CURSOR_DOWN:
			m.followSelectedPlaylist()
		case playlist.RENAME:
			selectedPlaylist := m.playlists.SelectedItem()
			if selectedPlaylist.Kind < playlist.USER {
				break
			}
			m.inputDialog.Title = "Rename playlist " + selectedPlaylist.Name
			m.inputDialog.SetValue(selectedPlaylist.Name)
			m.isRenamePlaylistActive = true
		case playlist.TOGGLE_VIEW:
			m.isPlaylistHideOverride = !m.isPlaylistHideOverride
		}

	// tracklist control update
	case tracklist.Control:
		switch msg {
		case tracklist.PLAY:
			playlistItem := m.playlists.SelectedItem()
			if !playlistItem.Active {
				break
			}
			if m.albumListActive() {
				cmd = m.openAlbum(m.tracklist.Index())
				cmds = append(cmds, cmd)
			} else {
				m.playSelectedPlaylist(m.playlistTrackIndex(m.tracklist.Index()))
			}
		case tracklist.CURSOR_UP, tracklist.CURSOR_DOWN:
			currentPlaylist := m.playlists.SelectedItem()
			cursorIndex := m.playlistTrackIndex(m.tracklist.Index())
			currentPlaylist.SelectedTrack = cursorIndex
			cmd = m.playlists.SetItem(m.playlists.Index(), currentPlaylist)
			cmds = append(cmds, cmd)
		case tracklist.LIKE:
			if !m.albumListActive() {
				cmd = m.likeSelectedTrack()
				cmds = append(cmds, cmd)
			}
		case tracklist.SHOW_ARTIST:
			if m.albumListActive() {
				break
			}
			track := m.tracklist.SelectedItem()
			if track.Track == nil {
				break
			}
			searchRes := api.SearchResult{
				Artists: api.SearchResultArtists{
					Results: track.Track.Artists,
				},
			}
			searchConf := config.Search{
				Artists: true,
			}
			m.isLoading = true
			go m.displaySearchResults(searchRes, &searchConf)
			cmds = append(cmds, m.spinner.Tick)
		case tracklist.ADD_TO_PLAYLIST:
			if m.albumListActive() {
				break
			}
			selectedTrack := m.tracklist.SelectedItem()
			m.searchDialog.Title = "Add " + selectedTrack.Track.Title + " to"
			m.searchDialog.Action = "add"
			m.isAddPlaylistActive = true
			m.Send(search.UPDATE_SUGGESTIONS)
		case tracklist.REMOVE_FROM_PLAYLIST:
			selectedPlaylist := m.playlists.SelectedItem()
			cmd = m.removeFromPlaylist(selectedPlaylist, m.playlistTrackIndex(m.tracklist.Index()))
			cmds = append(cmds, cmd)
		case tracklist.SEARCH:
			m.searchDialog.Title = "Search"
			m.searchDialog.Action = "search"
			m.isSearchActive = true
			m.Send(search.UPDATE_SUGGESTIONS)
		case tracklist.SHUFFLE:
			cmd = m.shufflePlaylist(m.playlists.SelectedItem())
			cmds = append(cmds, cmd)
		case tracklist.SHARE:
			if m.albumListActive() {
				break
			}
			link := api.ShareTrackLink(m.tracklist.SelectedItem().Track)
			if link != "" {
				m.clipboard.CopyText(link)
				m.showToast("Ссылка скопирована")
			}
		case tracklist.BACK:
			selectedPlaylist := m.playlists.SelectedItem()
			if len(selectedPlaylist.Albums) > 0 && selectedPlaylist.SelectedAlbum >= 0 {
				selectedPlaylist.SelectedTrack = selectedPlaylist.SelectedAlbum
				selectedPlaylist.SelectedAlbum = -1
				m.displayPlaylist(selectedPlaylist)
				m.indicateCurrentTrackPlaying(true)
				cmd = m.playlists.SetItem(m.playlists.Index(), selectedPlaylist)
				cmds = append(cmds, cmd)
			}
		}

	// player control update
	case tracker.Control:
		switch msg {
		case tracker.NEXT:
			m.nextTrack()
			m.showPlayingToast()
		case tracker.PREV:
			m.prevTrack()
			m.showPlayingToast()
		case tracker.LIKE:
			cmd = m.likePlayingTrack()
			cmds = append(cmds, cmd)
		case tracker.PLAY, tracker.PAUSE:
			m.mediaHandler.OnPlayPause()
			m.showPlayStateToast()
		case tracker.STOP:
			m.scrobbleCancel()
			m.mediaHandler.OnEnded()
		case tracker.REWIND:
			m.mediaHandler.OnSeek(m.tracker.Position())
			m.showSeekToast()
		case tracker.VOLUME:
			m.mediaHandler.OnVolume()
			m.showVolumeToast()
		case tracker.CACHE_TRACK:
			cmd = m.cacheCurrentTrack()
			cmds = append(cmds, cmd)
		case tracker.BUFFERING_COMPLETE:
			cacheMode := config.Current.CacheTracks
			if cacheMode == config.CACHE_ALL || (cacheMode == config.CACHE_LIKED_ONLY && m.likedTracksMap[m.tracker.CurrentTrack().Id]) {
				cmd = m.cacheCurrentTrack()
				cmds = append(cmds, cmd)
			}
		}

		m.tracker, cmd = m.tracker.Update(message)
		cmds = append(cmds, cmd)

	// search control update
	case search.Control:
		if m.isSearchActive {
			cmd = m.searchControl(msg)
			cmds = append(cmds, cmd)
		} else if m.isAddPlaylistActive {
			cmd = m.addPlaylistControl(msg)
			cmds = append(cmds, cmd)
		}

	// input dialog control update
	case input.Control:
		m.isRenamePlaylistActive = false
		cmd = m.renamePlaylistControl(msg)
		cmds = append(cmds, cmd)

	default:
		if m.isLoading {
			m.spinner, cmd = m.spinner.Update(message)
			cmds = append(cmds, cmd)
		} else if m.isSearchActive || m.isAddPlaylistActive {
			m.searchDialog, cmd = m.searchDialog.Update(message)
			cmds = append(cmds, cmd)
		} else if m.isRenamePlaylistActive {
			m.inputDialog, cmd = m.inputDialog.Update(message)
			cmds = append(cmds, cmd)
		} else {
			m.playlists, cmd = m.playlists.Update(message)
			cmds = append(cmds, cmd)
			m.tracklist, cmd = m.tracklist.Update(message)
			cmds = append(cmds, cmd)
			m.tracker, cmd = m.tracker.Update(message)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	var view string
	if m.contextMenu != nil {
		view = m.contextMenu.render(m.viewBody())
	} else {
		view = m.viewBody()
	}
	view = drawVersion(view)
	return m.drawToasts(view)
}

func (m *Model) viewBody() string {
	if m.isLoading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.spinner.View())
	}

	if m.isSettingsActive && m.settings != nil {
		return m.placeSettings()
	}

	if m.isHelpActive {
		return m.placeHelp()
	}

	if m.isWaveActive && m.wave != nil {
		return m.placeWaveSettings()
	}

	if m.eq != nil {
		return m.placeEqualizer()
	}

	if m.isStationsActive && m.stations != nil {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.stationsView())
	}

	if m.isQueueActive && m.queue != nil {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.queueView())
	}

	if m.isCacheActive && m.cache != nil {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.cacheView())
	}

	if m.isSearchActive || m.isAddPlaylistActive {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.searchDialog.View())
	} else if m.isRenamePlaylistActive {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renameView())
	}

	// the settings button lives at the bottom of the sidebar
	m.playlists.SetFooter(m.settingsButtonView())

	playlistView := m.playlists.View()
	playlistWidth := lipgloss.Width(playlistView)

	m.tracker.SetWidth(m.width - playlistWidth - 2)
	m.tracklist.SetWidth(m.width - playlistWidth - 2)

	trackerView := m.tracker.View()
	trackerHeight := lipgloss.Height(trackerView)
	m.tracklist.SetHeight(m.height - trackerHeight - 2)

	tracklistView := m.tracklist.View()

	var midPanel string
	if m.tracklist.Hidden {
		midPanel = trackerView
	} else if m.tracker.Hidden {
		midPanel = tracklistView
	} else {
		midPanel = lipgloss.JoinVertical(lipgloss.Left, tracklistView, trackerView)
	}

	m.updateMouseLayout(playlistView, tracklistView, trackerView, midPanel)

	return lipgloss.JoinHorizontal(lipgloss.Bottom, playlistView, midPanel)
}

//
// private methods
//

// followSelectedPlaylist displays tracks of the selected playlist.
func (m *Model) followSelectedPlaylist() {
	if len(m.playlists.Items()) == 0 {
		return
	}

	selectedPlaylist := m.playlists.SelectedItem()
	if selectedPlaylist == nil {
		return
	}

	items := m.playlists.Items()
	if m.currentPlaylistIndex >= 0 && m.currentPlaylistIndex < len(items) {
		currentPlaylist := items[m.currentPlaylistIndex]
		if selectedPlaylist.IsSame(currentPlaylist) && len(selectedPlaylist.Tracks) > 0 && m.currentAlbumIndex == selectedPlaylist.SelectedAlbum {
			selectedPlaylist.SelectedTrack = selectedPlaylist.CurrentTrack
			m.playlists.SetItem(m.playlists.Index(), selectedPlaylist)
		}
	} else {
		m.currentPlaylistIndex = -1
	}

	m.displayPlaylist(selectedPlaylist)
	m.indicateCurrentTrackPlaying(true)

	m.tracklist.Shufflable = (selectedPlaylist.Kind != playlist.NONE && selectedPlaylist.Kind != playlist.MYWAVE && len(selectedPlaylist.Tracks) > 0)

	// the same message is reused, so scrolling through the sidebar with the
	// wheel or the mouse does not leave a trail of toasts
	if selectedPlaylist.Name != m.toastPlaylist {
		m.toastPlaylist = selectedPlaylist.Name
		m.showPlaylistToast(selectedPlaylist.Name)
	}
}

func (m *Model) resize(width, height int) {
	m.width, m.height = width, height

	m.playlists.SetSize(style.SidePanelWidth, height-4)
	if !m.isPlaylistHideOverride {
		m.playlists.Hidden = m.width < style.SidePanelAutohide
	}

	searchWidth := style.SearchModalWidth
	if searchWidth > m.width {
		searchWidth = m.width - 2
	}

	m.searchDialog.SetSize(searchWidth, m.height-4)
	m.inputDialog.SetWidth(searchWidth)
}

func (m *Model) mediaHandle() {
	for msg := range m.mediaHandler.Message() {
		switch msg.Type {
		case handler.MSG_NEXT:
			m.Send(tracker.NEXT)
		case handler.MSG_PREVIOUS:
			m.Send(tracker.PREV)
		case handler.MSG_PLAY:
			m.tracker.Play()
			m.Send(tracker.PLAY)
		case handler.MSG_PAUSE:
			m.tracker.Pause()
			m.Send(tracker.PAUSE)
		case handler.MSG_PLAYPAUSE:
			if m.tracker.IsPlaying() {
				m.tracker.Pause()
				m.Send(tracker.PAUSE)
			} else {
				m.tracker.Play()
				m.Send(tracker.PLAY)
			}
		case handler.MSG_STOP:
			m.Send(tracker.STOP)
		case handler.MSG_SEEK:
			offset, ok := msg.Arg.(time.Duration)
			if ok {
				m.tracker.Rewind(offset)
			}
		case handler.MSG_SETPOS:
			pos, ok := msg.Arg.(time.Duration)
			if ok {
				m.tracker.SetPos(pos)
			}

		case handler.MSG_SET_SHUFFLE:
			val, ok := msg.Arg.(bool)
			if !ok || !val {
				break
			}
			currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
			if len(currentPlaylist.Tracks) == 0 {
				break
			}
			if currentPlaylist.Kind >= playlist.LIKES {
				cmd := m.shufflePlaylist(currentPlaylist)
				m.Send(func() tea.Cmd {
					return cmd
				})
			}
		case handler.MSG_SET_VOLUME:
			vol, ok := msg.Arg.(float64)
			if ok {
				m.tracker.SetVolume(vol)
			}

		case handler.MSG_GET_PLAYBACKSTATUS:
			var state handler.PlaybackState
			if m.tracker.IsPlaying() {
				state = handler.STATE_PLAYING
			} else {
				if m.tracker.IsStoped() {
					state = handler.STATE_STOPED
				} else {
					state = handler.STATE_PAUSED
				}
			}
			m.mediaHandler.SendAnswer(state)
		case handler.MSG_GET_SHUFFLE:
			m.mediaHandler.SendAnswer(false)
		case handler.MSG_GET_METADATA:
			if m.tracker.IsStoped() {
				m.mediaHandler.SendAnswer(handler.TrackMetadata{})
				break
			}
			track := m.tracker.CurrentTrack()
			artists := make([]string, 0, len(track.Artists))
			for i := range track.Artists {
				artists = append(artists, track.Artists[i].Name)
			}
			albumArtists := make([]string, 0)
			var albumName string
			genre := make([]string, 0)
			if len(track.Albums) != 0 {
				for i := range track.Albums[0].Artists {
					albumArtists = append(albumArtists, track.Albums[0].Artists[i].Name)
				}
				albumName = track.Albums[0].Title
				genre = append(genre, track.Albums[0].Genre)
			}

			md := handler.TrackMetadata{
				TrackId:      track.Id,
				Length:       time.Duration(track.DurationMs) * time.Millisecond,
				CoverUrl:     m.coverFilePath(track),
				AlbumName:    albumName,
				AlbumArtists: albumArtists,
				Artists:      artists,
				Genre:        genre,
				Title:        track.Title,
				Url:          api.ShareTrackLink(track),
			}
			m.mediaHandler.SendAnswer(md)
		case handler.MSG_GET_VOLUME:
			m.mediaHandler.SendAnswer(m.tracker.Volume())
		case handler.MSG_GET_POSITION:
			m.mediaHandler.SendAnswer(m.tracker.Position())
		}
	}
}

func (m *Model) coverFilePath(track *api.Track) string {
	tempDir := filepath.Join(os.TempDir(), config.DirName)
	if os.MkdirAll(tempDir, 0755) != nil {
		return ""
	}
	return filepath.Join(tempDir, track.Id+".jpg")
}

func (m *Model) metadataFilePath() string {
	tempDir := filepath.Join(os.TempDir(), config.DirName)
	if os.MkdirAll(tempDir, 0755) != nil {
		return ""
	}
	return filepath.Join(tempDir, "metadata.mp3")
}
