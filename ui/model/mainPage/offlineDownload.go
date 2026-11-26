package mainpage

import (
	"strconv"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/cache"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
)

// offlineDownloadMsg reports the progress of a playlist download. The
// generation makes the events of a cancelled download harmless.
type offlineDownloadMsg struct {
	generation int64
	name       string
	track      *api.Track
	done       int
	total      int
	failed     int
	finished   bool
}

// downloadPlaylistForOffline saves every track of the playlist that is not in
// the cache yet. It runs in one background goroutine, one track at a time, so
// the network and the disk are not flooded and the UI stays responsive.
func (m *Model) downloadPlaylistForOffline(pl *playlist.Item) tea.Cmd {
	if pl == nil || len(pl.Tracks) == 0 {
		return nil
	}
	client := m.client
	if client == nil {
		if config.Current.Offline {
			m.showToast("Офлайн-режим: скачивание недоступно")
		} else {
			m.showToast("Нет подключения к Яндексу")
		}
		return nil
	}

	pending := make([]api.Track, 0, len(pl.Tracks))
	for _, track := range pl.Tracks {
		if track.Id == "" || cache.Exists(track.Id) {
			continue
		}
		pending = append(pending, track)
	}
	if len(pending) == 0 {
		m.showToast("Уже сохранено: " + pl.Name)
		return nil
	}

	generation := atomic.AddInt64(&m.offlineDownloadGen, 1)
	name := pl.Name
	total := len(pending)
	m.showToast("Скачиваю для офлайна: " + name)

	go func() {
		failed := 0
		for i, track := range pending {
			if atomic.LoadInt64(&m.offlineDownloadGen) != generation {
				return
			}
			// the track is copied: the loop variable is reused
			current := track
			err := saveTrackToCache(client, &current)
			if err != nil {
				failed++
				log.Print(log.LVL_WARNIGN, "offline: failed to save track [%s]: %s", current.Id, err)
			}
			m.Send(offlineDownloadMsg{
				generation: generation,
				name:       name,
				track:      &current,
				done:       i + 1,
				total:      total,
				failed:     failed,
			})
		}
		m.Send(offlineDownloadMsg{
			generation: generation,
			name:       name,
			done:       total,
			total:      total,
			failed:     failed,
			finished:   true,
		})
	}()

	return nil
}

// applyOfflineDownload adds a saved track to the sidebar and reports the end
// of the playlist download.
func (m *Model) applyOfflineDownload(msg offlineDownloadMsg) tea.Cmd {
	if msg.generation != atomic.LoadInt64(&m.offlineDownloadGen) {
		return nil
	}

	if msg.track != nil && msg.track.Id != "" {
		m.addCachedTrack(msg.track)
	}
	if !msg.finished {
		return nil
	}

	saved := msg.total - msg.failed
	if msg.failed > 0 {
		m.showToast(msg.name + ": сохранено " + strconv.Itoa(saved) + " из " + strconv.Itoa(msg.total))
	} else {
		m.showToast(msg.name + ": сохранено " + strconv.Itoa(saved) + " — офлайн готов")
	}
	m.saveOfflineCatalog()
	return nil
}

// addCachedTrack puts a track that is already on the disk into the local
// playlist without re-reading the tags of the whole cache.
func (m *Model) addCachedTrack(track *api.Track) {
	if track == nil || track.Id == "" {
		return
	}
	if m.cachedTracksMap == nil {
		m.cachedTracksMap = make(map[string]bool)
	}
	if m.cachedTracksMap[track.Id] {
		return
	}
	m.cachedTracksMap[track.Id] = true

	local, index := m.playlists.GetFirst(playlist.LOCAL)
	if local == nil {
		return
	}
	local.AddTrack(track)
	m.playlists.SetItem(index, local)
	if m.playlists.SelectedItem() != nil && m.playlists.SelectedItem().Kind == playlist.LOCAL {
		m.displayPlaylist(local)
	}
}
