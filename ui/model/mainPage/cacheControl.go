package mainpage

import (
	"bytes"
	"errors"
	"io"
	"os"

	"github.com/bogem/id3v2/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/cache"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
)

func (m *Model) cacheCurrentTrack() tea.Cmd {
	currentTrack := m.tracker.CurrentTrack()
	if m.tracker.IsStoped() || currentTrack == nil || m.cachedTracksMap[currentTrack.Id] {
		return nil
	}
	reader := m.tracker.TrackBuffer()
	if reader == nil {
		return nil
	}
	track := *currentTrack
	metadataPath := m.metadataFilePath()

	// Parsing ID3 and copying a whole track to disk must not run in the UI loop.
	return func() tea.Msg {
		return bufferedTrackCachedMsg{
			track: &track,
			err:   saveBufferedTrack(reader, metadataPath, track.Id),
		}
	}
}

type bufferedTrackCachedMsg struct {
	track *api.Track
	err   error
}

func saveBufferedTrack(reader io.WriterTo, metadataPath, trackID string) error {
	metadataFile, err := os.Open(metadataPath)
	if err != nil {
		return err
	}
	defer metadataFile.Close()

	cacheFile, err := cache.Write(trackID)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			cache.Discard(cacheFile)
		}
	}()

	tag := id3v2.NewEmptyTag()
	tag.Reset(metadataFile, id3v2.Options{Parse: true})
	if _, err := tag.WriteTo(cacheFile); err != nil {
		return err
	}
	if _, err := reader.WriteTo(cacheFile); err != nil {
		return err
	}
	if err := cache.Commit(cacheFile, trackID); err != nil {
		return err
	}
	committed = true
	return nil
}

func (m *Model) applyBufferedTrackCached(msg bufferedTrackCachedMsg) tea.Cmd {
	if msg.track == nil {
		return nil
	}
	if msg.err != nil {
		log.Print(log.LVL_ERROR, "failed to save cached track: %s", msg.err)
		m.tracker.ShowError("cache write")
		return nil
	}

	m.cachedTracksMap[msg.track.Id] = true
	cachePlaylist, index := m.playlists.GetFirst(playlist.LOCAL)
	cachePlaylist.AddTrack(msg.track)
	cmd := m.playlists.SetItem(index, cachePlaylist)
	if m.playlists.SelectedItem().Kind == playlist.LOCAL {
		m.displayPlaylist(cachePlaylist)
	}
	m.showToast("В кэш: " + msg.track.Title)
	m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
	return cmd
}

func (m *Model) removeCache(track *api.Track) tea.Cmd {
	if m.tracker.CurrentTrack().Id == track.Id && len(m.tracker.CurrentTrack().RealId) == 0 {
		m.tracker.ShowError("can't remove currently playing track")
		return nil
	}

	err := cache.Remove(track.Id)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to remove cached file: %s", err)
		m.tracker.ShowError("cache remove")
		return nil
	}

	cachePlaylist, index := m.playlists.GetFirst(playlist.LOCAL)
	cachePlaylist.RemoveTrack(track.Id)

	delete(m.cachedTracksMap, track.Id)
	cmd := m.playlists.SetItem(index, cachePlaylist)

	if m.playlists.SelectedItem().Kind == playlist.LOCAL {
		m.displayPlaylist(cachePlaylist)
	}

	m.showToast("Удалено из кэша: " + track.Title)

	m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
	return cmd
}

// trackDownloadedMsg reports a track the user asked to save into the cache.
type trackDownloadedMsg struct {
	track *api.Track
	err   error
}

// downloadTrack saves the track into the cache in the background, so the
// window stays responsive while the track is on its way.
func (m *Model) downloadTrack(track *api.Track) tea.Cmd {
	if track == nil || m.client == nil {
		return nil
	}
	if m.cachedTracksMap[track.Id] {
		m.showToast("Уже в кэше: " + track.Title)
		return nil
	}
	if m.downloadingTracks[track.Id] {
		m.showToast("Уже скачивается: " + track.Title)
		return nil
	}

	m.downloadingTracks[track.Id] = true
	m.showToast("Скачиваю: " + track.Title)

	client := m.client
	return func() tea.Msg {
		return trackDownloadedMsg{track: track, err: saveTrackToCache(client, track)}
	}
}

// saveTrackToCache downloads the track and writes it to the cache together
// with its tags and its cover. It runs off the UI loop: it must not touch the
// state of the window.
func saveTrackToCache(client *api.YaMusicClient, track *api.Track) error {
	var (
		coverType  string
		coverBytes []byte
	)
	// a broken cover must not lose the track itself
	var coverBuf bytes.Buffer
	if cType, err := api.DownloadTrackCover(&coverBuf, track, 200); err != nil {
		log.Print(log.LVL_WARNIGN, "unable to download track [%s] cover: %s", track.Id, err)
	} else {
		coverType = cType
		coverBytes = coverBuf.Bytes()
	}

	trackInfos, err := client.TrackDownloadInfo(track.Id)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to obtain track [%s] info: %s", track.Id, err)
		return err
	}
	var bestInfo api.TrackDownloadInfo
	var bestBitrate int
	for _, ti := range trackInfos {
		if ti.BbitrateInKbps > bestBitrate {
			bestBitrate = ti.BbitrateInKbps
			bestInfo = ti
		}
	}

	reader, size, err := client.DownloadTrack(bestInfo)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to download track [%s]: %s", track.Id, err)
		return err
	}
	defer reader.Close()
	if size <= 0 {
		return errors.New("empty track")
	}

	cacheFile, err := cache.Write(track.Id)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to write cache file: %s", err)
		return err
	}
	committed := false
	defer func() {
		if !committed {
			cache.Discard(cacheFile)
		}
	}()

	if _, err := buildTrackTag(track, coverType, coverBytes).WriteTo(cacheFile); err != nil {
		log.Print(log.LVL_ERROR, "failed to write tags of track [%s]: %s", track.Id, err)
		return err
	}
	if _, err := io.Copy(cacheFile, reader); err != nil {
		log.Print(log.LVL_ERROR, "failed to save track [%s]: %s", track.Id, err)
		return err
	}
	// the file becomes a cached track only when it is complete
	if err := cache.Commit(cacheFile, track.Id); err != nil {
		log.Print(log.LVL_ERROR, "failed to commit cache file: %s", err)
		return err
	}
	committed = true
	return nil
}

// applyDownloadedTrack puts a track that was just saved into the cached
// playlist.
func (m *Model) applyDownloadedTrack(msg trackDownloadedMsg) tea.Cmd {
	if msg.track == nil {
		return nil
	}
	delete(m.downloadingTracks, msg.track.Id)

	if msg.err != nil {
		m.showToast("Не удалось скачать: " + msg.track.Title)
		return nil
	}
	m.cachedTracksMap[msg.track.Id] = true
	if cachePlaylist, index := m.playlists.GetFirst(playlist.LOCAL); cachePlaylist != nil {
		cachePlaylist.AddTrack(msg.track)
		cmd := m.playlists.SetItem(index, cachePlaylist)
		if m.playlists.SelectedItem().Kind == playlist.LOCAL {
			m.displayPlaylist(cachePlaylist)
		}
		m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
		m.showToast("Скачано: " + msg.track.Title)
		return cmd
	}
	return nil
}
