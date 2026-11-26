package mainpage

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
)

// artistTracksMsg carries the popular tracks of an artist back to the UI
// loop. The request runs in the background; the sidebar and the player are
// only touched on the UI goroutine.
type artistTracksMsg struct {
	name   string
	tracks []api.Track
	err    error
}

// showArtist opens the popular tracks of the first artist of the track. It is
// used for the artists with a big discography.
func (m *Model) showArtist(track *api.Track) tea.Cmd {
	if track == nil || len(track.Artists) == 0 || track.Artists[0].Id == 0 {
		return nil
	}
	client := m.client
	if client == nil {
		return nil
	}

	artist := track.Artists[0]
	m.isLoading = true

	go func() {
		defer m.Send(LOADING_DONE)

		popular, err := client.ArtistPopularTracks(uint64(artist.Id))
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to obtain artist [%s] tracks: %s", artist.Name, err)
			m.Send(artistTracksMsg{name: artist.Name, err: err})
			return
		}

		ids := make([]string, len(popular.Tracks))
		copy(ids, popular.Tracks)

		tracks, err := client.Tracks(ids)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to obtain artist [%s] tracks info: %s", artist.Name, err)
			m.Send(artistTracksMsg{name: artist.Name, err: err})
			return
		}

		m.Send(artistTracksMsg{name: artist.Name, tracks: tracks})
	}()

	return m.spinner.Tick
}

// applyArtistTracks adds the loaded artist list to the sidebar.
func (m *Model) applyArtistTracks(msg artistTracksMsg) tea.Cmd {
	if msg.err != nil || len(msg.tracks) == 0 {
		m.tracker.ShowError("artist tracks")
		return nil
	}

	m.insertSidebarItem(&playlist.Item{
		Name:    msg.name + " popular",
		Active:  true,
		Subitem: true,
		Label:   "артист",
		Tracks:  msg.tracks,
	})
	return nil
}

// insertSidebarItem appends an item to the end of the playlists sidebar.
func (m *Model) insertSidebarItem(item *playlist.Item) {
	items := m.playlists.Items()
	items = append(items, item)

	m.playlists.SetItems(items)
	m.playlists.Select(len(items) - 1)
	m.followSelectedPlaylist()
}
