package mainpage

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dece2183/yamusic-tui/api"
	"github.com/dece2183/yamusic-tui/log"
	"github.com/dece2183/yamusic-tui/ui/components/playlist"
)

// showArtist adds the popular tracks of the track artist to the sidebar.
//
// It intentionally loads only the popular tracks: walking through all
// the artist albums requires one request per album, which takes minutes
// for the artists with a big discography.
func (m *Model) showArtist(track *api.Track) tea.Cmd {
	if track == nil || len(track.Artists) == 0 || track.Artists[0].Id == 0 {
		return nil
	}
	if m.client == nil {
		return nil
	}

	artist := track.Artists[0]
	m.isLoading = true

	go func() {
		defer m.Send(LOADING_DONE)

		popular, err := m.client.ArtistPopularTracks(artist.Id)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to obtain artist [%s] tracks: %s", artist.Name, err)
			m.tracker.ShowError("artist tracks")
			return
		}

		ids := make([]string, len(popular.Tracks))
		copy(ids, popular.Tracks)

		tracks, err := m.client.Tracks(ids)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to obtain artist [%s] tracks info: %s", artist.Name, err)
			m.tracker.ShowError("artist tracks info")
			return
		}

		if len(tracks) == 0 {
			m.tracker.ShowError("artist tracks")
			return
		}

		m.insertSidebarItem(&playlist.Item{
			Name:    artist.Name + " popular",
			Active:  true,
			Subitem: true,
			Tracks:  tracks,
		})
	}()

	return m.spinner.Tick
}

// insertSidebarItem appends an item to the end of the playlists sidebar.
func (m *Model) insertSidebarItem(item *playlist.Item) {
	items := m.playlists.Items()
	items = append(items, item)

	m.playlists.SetItems(items)
	m.playlists.Select(len(items) - 1)
	m.followSelectedPlaylist()
}
