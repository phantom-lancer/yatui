package mainpage

import (
	"errors"
	"net/url"
	"strconv"
	"sync"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/cache"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
)

type LoadingMsg uint

const (
	LOADING_DONE LoadingMsg = iota
)

type menuBlock struct {
	items []*playlist.Item
	err   error
}

func (m *Model) initialLoad() {
	// Only one full load may run at a time. A second one waits: the network
	// phase can take seconds, and two loads would rewrite the sidebar and the
	// client concurrently.
	m.loadMu.Lock()
	defer m.loadMu.Unlock()

	m.Send(uiHideErrorMsg{})
	m.playlists.Reset()

	// The offline mode never touches the network: no client is created, no
	// wave session, no likes and no playlists are requested. Only the tracks
	// that are already in the cache are listed.
	if config.Current.Offline {
		m.client = nil
		m.loadOfflineCatalog()
		return
	}

	if len(config.Current.Token) == 0 {
		log.Print(log.LVL_ERROR, "missing client token, check the config file at '%s'", config.Path())
		m.reportError("missing token")
		m.client = nil
	} else {
		c, err := api.NewClient(config.DirName, config.Current.Token)
		m.client = c
		if err != nil {
			if _, ok := err.(*url.Error); ok {
				log.Print(log.LVL_ERROR, "failed to connect to the Yandex server: %s", err)
				m.reportError("unable to connect to the Yandex server")
			} else {
				log.Print(log.LVL_ERROR, "client init error: %s", err)
				m.reportError("Не удалось войти: " + err.Error() + " — проверьте сеть или токен (yatui --login)")
			}
		}
	}

	var (
		wg                      sync.WaitGroup
		myWaveMenuBlock         menuBlock
		localTracksMenuBlock    menuBlock
		likedTracksMenuBlock    menuBlock
		likedPlaylistsMenuBlock menuBlock
		likedArtistsMenuBlock   menuBlock
		likedAlbumsMenuBlock    menuBlock
		pinnedAlbumsMenuBlock   menuBlock
		userPlaylistsMenuBlock  menuBlock
	)

	// The wave settings and the client are read once, before the workers
	// start: changing them in a dialog or switching the offline mode must not
	// race with the session and playlist requests.
	seeds := m.waveSeeds()
	client := m.client

	wg.Add(8)
	go m.loadMyWave(&wg, &myWaveMenuBlock, client, seeds)
	go m.loadLocalTracks(&wg, &localTracksMenuBlock)
	go m.loadLikedTracks(&wg, &likedTracksMenuBlock, client)
	go m.loadLikedPlaylists(&wg, &likedPlaylistsMenuBlock, client)
	go m.loadLikedArtists(&wg, &likedArtistsMenuBlock, client)
	go m.loadLikedAlbums(&wg, &likedAlbumsMenuBlock, client)
	go m.loadPinnedAlbums(&wg, &pinnedAlbumsMenuBlock, client)
	go m.loadUserPlaylists(&wg, &userPlaylistsMenuBlock, client)
	wg.Wait()

	if myWaveMenuBlock.err == nil {
		for _, item := range myWaveMenuBlock.items {
			m.playlists.InsertItem(-1, item)
		}
	} else {
		log.Print(log.LVL_ERROR, "unable to init rotor session: %s", myWaveMenuBlock.err)
		m.reportError("unable to init rotor session")
	}

	if localTracksMenuBlock.err == nil {
		for _, item := range localTracksMenuBlock.items {
			m.playlists.InsertItem(-1, item)
			for _, tr := range item.Tracks {
				m.cachedTracksMap[tr.Id] = true
			}
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to list cached tracks: %s", localTracksMenuBlock.err)
		m.reportError("cache list")
	}

	m.playlists.InsertItem(-1, playlist.ItemEmpty())
	m.playlists.InsertItem(-1, playlist.ItemCategory("likes:"))

	if likedTracksMenuBlock.err == nil {
		for _, item := range likedTracksMenuBlock.items {
			m.playlists.InsertItem(-1, item)
			for _, tr := range item.Tracks {
				m.likedTracksMap[tr.Id] = true
			}
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to obtain liked tracks: %s", likedTracksMenuBlock.err)
		m.reportError("liked tracks")
	}

	if likedPlaylistsMenuBlock.err == nil {
		for _, item := range likedPlaylistsMenuBlock.items {
			m.playlists.InsertItem(-1, item)
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to obtain liked playlists: %s", likedPlaylistsMenuBlock.err)
		m.reportError("liked playlists")
	}

	if likedArtistsMenuBlock.err == nil {
		for _, item := range likedArtistsMenuBlock.items {
			m.playlists.InsertItem(-1, item)
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to obtain liked artists: %s", likedArtistsMenuBlock.err)
		m.reportError("liked artists")
	}

	if likedAlbumsMenuBlock.err == nil {
		for _, item := range likedAlbumsMenuBlock.items {
			m.playlists.InsertItem(-1, item)
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to obtain liked albums: %s", likedAlbumsMenuBlock.err)
		m.reportError("liked albums")
	}

	if pinnedAlbumsMenuBlock.err == nil {
		for _, item := range pinnedAlbumsMenuBlock.items {
			m.playlists.InsertItem(-1, item)
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to obtain pinned albums: %s", pinnedAlbumsMenuBlock.err)
		m.reportError("pinned albums")
	}

	if userPlaylistsMenuBlock.err == nil {
		for _, item := range userPlaylistsMenuBlock.items {
			m.playlists.InsertItem(-1, item)
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to obtain user playlists: %s", userPlaylistsMenuBlock.err)
		m.reportError("playlists")
	}

	m.currentPlaylistIndex = -1
	m.playlists.Select(0)
	// the catalog is refreshed after every start: it is what the offline
	// mode lists when the network is gone
	m.saveOfflineCatalog()
	m.Send(LOADING_DONE)
}

// loadOfflineCatalog fills the sidebar with the cached tracks and the
// playlists whose tracks were saved for the offline mode.
func (m *Model) loadOfflineCatalog() {
	localTracks, err := cache.ListTracks()
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to list cached tracks: %s", err)
		m.reportError("cache list")
	}

	playlists, err := cache.LoadOfflineCatalog()
	if err != nil {
		log.Print(log.LVL_WARNIGN, "failed to read the offline catalog: %s", err)
	}

	// the local list is always first: it is the one thing that cannot be
	// missing in the offline mode
	m.playlists.InsertItem(-1, &playlist.Item{
		Name:   "local",
		Kind:   playlist.LOCAL,
		Active: true,
		Tracks: localTracks,
	})
	for _, tr := range localTracks {
		m.cachedTracksMap[tr.Id] = true
	}

	seen := map[string]bool{"local": true}
	for _, saved := range playlists {
		if saved.Name == "" || seen[saved.Name] || len(saved.Tracks) == 0 {
			continue
		}
		seen[saved.Name] = true
		m.playlists.InsertItem(-1, &playlist.Item{
			Name:   saved.Name,
			Kind:   saved.Kind,
			Active: true,
			Tracks: saved.Tracks,
		})
		for _, tr := range saved.Tracks {
			m.cachedTracksMap[tr.Id] = true
		}
	}

	m.currentPlaylistIndex = -1
	m.playlists.Select(0)
	if len(localTracks) == 0 && len(playlists) == 0 {
		m.reportError("офлайн: в кэше нет треков")
	}
	m.Send(LOADING_DONE)
}

// offlineCatalog takes a copy of the playlists for the writer. It runs on the
// goroutine that owns the sidebar, so the background write never reads a list
// the UI is changing.
func (m *Model) offlineCatalog() []cache.OfflinePlaylist {
	items := m.playlists.Items()
	playlists := make([]cache.OfflinePlaylist, 0, len(items))
	for _, item := range items {
		if item == nil || len(item.Tracks) == 0 {
			continue
		}
		tracks := make([]api.Track, len(item.Tracks))
		copy(tracks, item.Tracks)
		playlists = append(playlists, cache.OfflinePlaylist{
			Name:   item.Name,
			Kind:   item.Kind,
			Tracks: tracks,
		})
	}
	return playlists
}

// saveOfflineCatalog writes the playlists and their cached tracks to the
// offline catalog. The disk work runs in the background: the sidebar must not
// wait for it.
func (m *Model) saveOfflineCatalog() {
	playlists := m.offlineCatalog()
	go func() {
		if err := cache.SaveOfflineCatalog(playlists); err != nil {
			log.Print(log.LVL_WARNIGN, "failed to save the offline catalog: %s", err)
		}
	}()
}

func (m *Model) loadMyWave(wg *sync.WaitGroup, block *menuBlock, client *api.YaMusicClient, seeds []string) {
	defer wg.Done()

	if client == nil {
		return
	}

	session, err := client.RotorNewSessionWithSeeds(seeds)
	if err != nil {
		block.err = err
		return
	}

	st := &playlist.Item{Name: "my wave", Kind: playlist.MYWAVE, Active: true, Subitem: false, Rotor: true}
	st.SessionId = session.RadioSessionId
	st.SessionBatch = session.BatchId
	if len(session.AcceptedSeeds) > 0 {
		st.StationId = session.AcceptedSeeds[0]
	} else {
		st.StationId = session.Id
	}
	if len(session.Sequence) > 0 {
		st.Tracks = []api.Track{session.Sequence[0].Track}
	} else {
		block.err = errors.New("unable to get session tracks")
	}

	block.items = append(block.items, st)
}

func (m *Model) loadLocalTracks(wg *sync.WaitGroup, block *menuBlock) {
	defer wg.Done()

	localTracks, err := cache.ListTracks()
	if err != nil {
		block.err = err
		return
	}

	st := &playlist.Item{Name: "local", Kind: playlist.LOCAL, Active: true, Subitem: false, Tracks: localTracks}
	block.items = append(block.items, st)
}

func (m *Model) loadLikedTracks(wg *sync.WaitGroup, block *menuBlock, client *api.YaMusicClient) {
	defer wg.Done()

	if client == nil {
		return
	}

	likes, err := client.LikedTracks()
	if err != nil {
		block.err = err
		return
	}

	ids := make([]string, len(likes))
	for i, tr := range likes {
		ids[i] = tr.Id
	}

	tracks, err := client.Tracks(ids)
	if err != nil {
		block.err = err
		return
	}

	st := &playlist.Item{Name: "tracks", Kind: playlist.LIKES, Active: len(tracks) > 0, Subitem: true, Tracks: tracks}
	block.items = append(block.items, st)
}

func (m *Model) loadLikedPlaylists(wg *sync.WaitGroup, block *menuBlock, client *api.YaMusicClient) {
	defer wg.Done()

	if client == nil {
		return
	}

	liked, err := client.LikedPlaylists()
	if err != nil {
		block.err = err
		return
	}

	for _, info := range liked {
		if !info.Playlist.Available {
			continue
		}

		tracks, err := client.PlaylistTracks(info.Playlist.Kind, info.Playlist.Owner.Uid, false)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to obtain the tracks of the liked playlist [%s]: %s", info.Playlist.Title, err)
			continue
		}

		name := info.Playlist.Title
		if name == "" {
			name = "playlist"
		}

		st := &playlist.Item{
			Name:     name,
			Kind:     playlist.LIKES_PLAYLISTS,
			Active:   len(tracks) > 0,
			Subitem:  true,
			Tracks:   tracks,
			Revision: info.Playlist.Revision,
		}
		block.items = append(block.items, st)
	}
}

func (m *Model) loadLikedArtists(wg *sync.WaitGroup, block *menuBlock, client *api.YaMusicClient) {
	defer wg.Done()

	if client == nil {
		return
	}

	liked, err := client.LikedArtists()
	if err != nil {
		block.err = err
		return
	}

	const (
		maxArtists         = 10
		maxTracksPerArtist = 20
		maxTracks          = 100
	)

	ids := make([]string, 0, 64)
	for i, info := range liked {
		if i >= maxArtists || len(ids) >= maxTracks {
			break
		}

		id, err := strconv.ParseUint(info.Id, 10, 64)
		if err != nil {
			continue
		}

		popular, err := client.ArtistPopularTracks(id)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to obtain the tracks of the liked artist [%s]: %s", info.Name, err)
			continue
		}

		for j, track := range popular.Tracks {
			if j >= maxTracksPerArtist || len(ids) >= maxTracks {
				break
			}
			ids = append(ids, track)
		}
	}

	tracks := make([]api.Track, 0)
	if len(ids) > 0 {
		tracks, err = client.Tracks(ids)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to obtain the tracks of the liked artists: %s", err)
			tracks = nil
		}
	}

	st := &playlist.Item{Name: "artists", Kind: playlist.LIKES_ARTISTS, Active: len(tracks) > 0, Subitem: true, Tracks: tracks}
	block.items = append(block.items, st)
}

func (m *Model) loadLikedAlbums(wg *sync.WaitGroup, block *menuBlock, client *api.YaMusicClient) {
	defer wg.Done()

	if client == nil {
		return
	}

	likedAlbums, err := client.LikedAlbums()
	if err != nil {
		block.err = err
		return
	}

	albums := make([]api.Album, 0, len(likedAlbums))
	for _, albumInfo := range likedAlbums {
		album, err := client.Album(albumInfo.Id, true)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to obtain album [%d] info: %s", albumInfo.Id, err)
			continue
		}
		albums = append(albums, album)
	}

	st := &playlist.Item{Name: "albums", Kind: playlist.ALBUMS, Active: len(albums) > 0, Subitem: true, Albums: albums, SelectedAlbum: -1}
	block.items = append(block.items, st)
}

func (m *Model) loadPinnedAlbums(wg *sync.WaitGroup, block *menuBlock, client *api.YaMusicClient) {
	defer wg.Done()

	if client == nil {
		return
	}

	pinnedAlbums, err := client.PinnedAlbums()
	if err != nil {
		block.err = err
		return
	}

	albums := make([]api.Album, 0, len(pinnedAlbums))
	for _, albumInfo := range pinnedAlbums {
		album, err := client.Album(uint64(albumInfo.Data.Id), true)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to obtain pinned album [%d] info: %s", uint64(albumInfo.Data.Id), err)
			continue
		}
		albums = append(albums, album)
	}

	if len(albums) > 0 {
		block.items = append(block.items, playlist.ItemEmpty())
		block.items = append(block.items, playlist.ItemCategory("pins:"))
		block.items = append(block.items, pinnedAlbumItems(albums)...)
	}
}

func (m *Model) loadUserPlaylists(wg *sync.WaitGroup, block *menuBlock, client *api.YaMusicClient) {
	defer wg.Done()

	if client == nil {
		return
	}

	playlists, err := client.ListPlaylists()
	if err != nil {
		block.err = err
		return
	}

	playlistTracks := make([][]api.Track, len(playlists))
	var innerWg sync.WaitGroup
	for i, pl := range playlists {
		innerWg.Add(1)
		go func(i int, pl api.Playlist) {
			defer innerWg.Done()
			tracks, terr := client.PlaylistTracks(pl.Kind, pl.Owner.Uid, false)
			if terr != nil {
				log.Print(log.LVL_ERROR, "failed to obtain user playlist [%s] tracks: %s", pl.Title, terr)
				return
			}
			playlistTracks[i] = tracks
		}(i, pl)
	}
	innerWg.Wait()

	if len(playlists) > 0 {
		block.items = append(block.items, playlist.ItemEmpty())
		block.items = append(block.items, playlist.ItemCategory("playlists:"))

		for i, pl := range playlists {
			tracks := playlistTracks[i]
			if len(tracks) == 0 {
				m.reportError("playlist tracks")
				continue
			}
			block.items = append(block.items, &playlist.Item{
				Name:     pl.Title,
				Kind:     pl.Kind,
				Revision: pl.Revision,
				Active:   true,
				Subitem:  true,
				Tracks:   tracks,
			})
		}
	}
}

// pinnedAlbumItems creates one playlist entry per pinned album. Kept separate
// from the API loading so the grouping can be tested without network access.
func pinnedAlbumItems(albums []api.Album) []*playlist.Item {
	items := make([]*playlist.Item, 0, len(albums))
	for _, album := range albums {
		var tracks []api.Track
		for _, volume := range album.Volumes {
			tracks = append(tracks, volume...)
		}
		if len(tracks) == 0 {
			continue
		}
		items = append(items, &playlist.Item{
			Name:    album.Title,
			Kind:    playlist.ALBUMS,
			Active:  true,
			Subitem: true,
			Tracks:  tracks,
		})
	}
	return items
}
