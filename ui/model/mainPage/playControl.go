package mainpage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	_ "image/jpeg"
	_ "image/png"

	"github.com/bogem/id3v2/v2"
	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/cache"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/stream"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
	"github.com/phantom-lancer/yatui/ui/components/tracker"
	"github.com/phantom-lancer/yatui/ui/components/tracklist"
	"github.com/phantom-lancer/yatui/ui/helpers"
)

const (
	_TRACK_DOWNLOAD_TRIES     = 3
	_TRACK_FINISHED_THRESHOLD = 0.8
	// _TRACK_FAILURES_LIMIT is how many broken tracks in a row are skipped
	// before the playback gives up.
	_TRACK_FAILURES_LIMIT = 3
	// coverMaxAge is how long the shared cover file stays valid. Playlist
	// covers change now and then, so the file is refreshed once a week.
	coverMaxAge = 7 * 24 * time.Hour
)

func (m *Model) feedbackOnTrack(batch string) *api.RotorFeedback {
	currTrack := m.tracker.CurrentTrack()
	if currTrack == nil {
		return nil
	}
	var evType api.TrackEventType
	if m.tracker.Progress() > _TRACK_FINISHED_THRESHOLD {
		evType = api.EV_TRACK_FINISHED
	} else {
		evType = api.EV_TRACK_SKIPED
	}
	ev := api.NewTrackFeedbackEvent(evType, currTrack, m.tracker.Playtime().Seconds())
	fb := api.NewFeedback(batch, ev)
	log.Print(log.LVL_INFO, "feedback event sended: "+ev.Type+" track: "+currTrack.Title)
	return fb
}

func (m *Model) rotateTracks(pl *playlist.Item) {
	if !pl.Rotor || m.client == nil {
		return
	}

	suggestedTracks, err := m.client.RotorSessionTracks(pl.SessionId, []*api.RotorFeedback{}, pl.Tracks)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to obtain more rotor tracks: %s", err)
		m.tracker.ShowError("next track obtain failure")
		return
	}
	if len(suggestedTracks.Sequence) == 0 {
		log.Print(log.LVL_WARNIGN, "the radio session returned no tracks")
		return
	}
	nextTrack := suggestedTracks.Sequence[0].Track

	pl.SessionBatch = suggestedTracks.BatchId
	pl.Tracks = append(pl.Tracks, nextTrack)
	if m.currentPlaylistIndex >= 0 && m.playlists.Items()[m.currentPlaylistIndex].IsSame(pl) {
		m.playQueue = pl.Tracks
	}

	if m.playlists.SelectedItem().IsSame(pl) {
		tackItems := m.tracklist.Items()
		if len(tackItems) > 0 {
			lastTrack := tackItems[len(tackItems)-1]
			lastTrack.IsSuggestion = false
			m.tracklist.SetItem(len(tackItems)-1, lastTrack)
		}
		m.tracklist.InsertItem(-1, tracklist.Item{
			Track:        &pl.Tracks[len(pl.Tracks)-1],
			Artists:      helpers.ArtistList(nextTrack.Artists),
			IsSuggestion: true,
		})
	}
}

func (m *Model) prevTrack() {
	if m.manualPlaying != nil {
		// a manual track is playing: back returns to the playlist track it
		// interrupted
		m.manualPlaying = nil
		m.refreshQueue()
		if m.playQueueTrackIndex >= 0 && m.playQueueTrackIndex < len(m.playQueue) {
			if track := &m.playQueue[m.playQueueTrackIndex]; track.Available {
				m.playTrack(track)
				return
			}
		}
		m.Send(tracker.STOP)
		return
	}
	if len(m.playQueue) == 0 || m.playQueueTrackIndex == 0 {
		m.Send(tracker.STOP)
		return
	}

	m.playQueueTrackIndex--
	for m.playQueueTrackIndex > 0 && !m.playQueue[m.playQueueTrackIndex].Available {
		m.playQueueTrackIndex--
	}

	if m.currentPlaylistIndex >= 0 {
		m.indicateCurrentTrackPlaying(false)
		currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
		if currentPlaylist.Rotor && m.tracker.IsPlaying() {
			go m.client.RotorSessionFeedback(currentPlaylist.SessionId, m.feedbackOnTrack(currentPlaylist.SessionBatch))
		}

		selectedPlaylist := m.playlists.SelectedItem()
		shouldFollow := currentPlaylist.IsSame(selectedPlaylist) && m.playlistTrackIndex(m.tracklist.Index()) == currentPlaylist.CurrentTrack
		currentPlaylist.CurrentTrack = m.playQueueTrackIndex
		if shouldFollow {
			m.tracklist.Select(m.tracklistIndex(currentPlaylist.CurrentTrack))
			currentPlaylist.SelectedTrack = currentPlaylist.CurrentTrack
		}

		m.playlists.SetItem(m.currentPlaylistIndex, currentPlaylist)
	}

	track := &m.playQueue[m.playQueueTrackIndex]
	if !track.Available {
		m.Send(tracker.STOP)
		return
	}

	m.playTrack(track)
}

func (m *Model) nextTrack() {
	// tracks explicitly put into the queue always go first
	for len(m.manualQueue) > 0 {
		track := m.manualQueue[0]
		m.manualQueue = m.manualQueue[1:]
		if track.Available {
			// the track leaves the queue: the list and the "next" line are
			// refreshed, and the list shows it as the playing one
			m.manualPlaying = &track
			m.refreshQueue()
			m.playTrack(&track)
			return
		}
	}

	if len(m.playQueue) == 0 {
		m.Send(tracker.STOP)
		return
	}

	// a track of the playlist starts: the manual one, if any, is over
	m.manualPlaying = nil

	// the wave has to be asked for more tracks before the queue runs out:
	// the current track may be the last one, e.g. when the user rearranged
	// the list by hand, and then the wave used to stop instead of going on
	rotated := false
	if m.currentPlaylistIndex >= 0 && m.currentPlaylistIndex < len(m.playlists.Items()) {
		currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
		if currentPlaylist.Rotor && m.playQueueTrackIndex >= len(m.playQueue)-1 {
			m.rotateTracks(currentPlaylist)
			rotated = true
		}
	}

	m.playQueueTrackIndex++
	for m.playQueueTrackIndex < len(m.playQueue)-1 && !m.playQueue[m.playQueueTrackIndex].Available {
		m.playQueueTrackIndex++
	}

	if m.currentPlaylistIndex >= 0 {
		m.indicateCurrentTrackPlaying(false)
		currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
		if currentPlaylist.Rotor && m.tracker.IsPlaying() {
			go m.client.RotorSessionFeedback(currentPlaylist.SessionId, m.feedbackOnTrack(currentPlaylist.SessionBatch))
		}

		selectedPlaylist := m.playlists.SelectedItem()
		shouldFollow := currentPlaylist.IsSame(selectedPlaylist) && m.playlistTrackIndex(m.tracklist.Index()) == currentPlaylist.CurrentTrack
		currentPlaylist.CurrentTrack = m.playQueueTrackIndex
		if shouldFollow {
			m.tracklist.Select(m.tracklistIndex(currentPlaylist.CurrentTrack))
			currentPlaylist.SelectedTrack = currentPlaylist.CurrentTrack
		}

		if !rotated && currentPlaylist.CurrentTrack == len(m.playQueue)-1 {
			m.rotateTracks(currentPlaylist)
		}

		m.playlists.SetItem(m.currentPlaylistIndex, currentPlaylist)
	}

	if m.playQueueTrackIndex >= len(m.playQueue) {
		// the queue ended: repeat the track that just played, wrap to the
		// start, or stop
		m.playQueueTrackIndex = repeatWrapIndex(len(m.playQueue), m.repeatMode)
		if m.playQueueTrackIndex < 0 {
			m.Send(tracker.STOP)
			return
		}
		if m.repeatMode == tracker.RepeatAll {
			for m.playQueueTrackIndex < len(m.playQueue)-1 &&
				!m.playQueue[m.playQueueTrackIndex].Available {
				m.playQueueTrackIndex++
			}
		}
	}

	track := &m.playQueue[m.playQueueTrackIndex]
	if !track.Available {
		m.Send(tracker.STOP)
		return
	}

	m.playTrack(track)
}

// replayCurrentTrack starts the playing track over: this is what "repeat one"
// does when the track reaches its end. A track the user put in the queue
// repeats itself too, even though it is not a part of the playlist.
func (m *Model) replayCurrentTrack() {
	if m.manualPlaying != nil {
		track := *m.manualPlaying
		m.playTrack(&track)
		return
	}

	if m.playQueueTrackIndex < 0 || m.playQueueTrackIndex >= len(m.playQueue) {
		m.Send(tracker.STOP)
		return
	}

	track := &m.playQueue[m.playQueueTrackIndex]
	if !track.Available {
		m.Send(tracker.STOP)
		return
	}

	m.playTrack(track)
}

// trackReadyMsg carries a prepared track back to the UI loop. The download
// runs in its own goroutine: a slow network must never freeze the window.
type trackReadyMsg struct {
	generation int64
	track      *api.Track
	buffer     *stream.BufferedStream
	fromCache  bool
	lyrics     []api.LyricPair
	err        error
}

func (m *Model) playTrack(track *api.Track) {
	m.tracker.Stop()
	// a new track drops the download that may still be in flight
	m.playGeneration++
	m.loadingTrackId = track.Id
	m.tracker.LoadTrack(track)

	go m.loadTrack(m.playGeneration, track)
}

// loadTrack downloads everything the track needs and hands it to the UI loop.
func (m *Model) loadTrack(generation int64, track *api.Track) {
	var (
		wg sync.WaitGroup

		coverType  string
		coverBytes []byte

		lyrics []api.LyricPair

		trackReader    io.ReadCloser
		trackSize      int64
		trackFromCache bool
		downloadErr    error
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		coverPath := m.coverFilePath(track)
		coverFile, ferr := os.OpenFile(coverPath, os.O_CREATE|os.O_RDWR, 0755)
		if ferr != nil {
			log.Print(log.LVL_WARNIGN, "unable to open cover file [%s]: %s", coverPath, ferr)
			return
		}
		defer coverFile.Close()
		stat, serr := coverFile.Stat()
		// a cover file is reused while it is fresh, so the playlist covers
		// of the past are not downloaded again and again
		fresh := serr == nil && stat.Size() > 0 && time.Since(stat.ModTime()) < coverMaxAge
		if !fresh {
			coverFile.Truncate(0)
			coverFile.Seek(0, io.SeekStart)
			cType, derr := api.DownloadTrackCover(coverFile, track, 200)
			if derr != nil {
				log.Print(log.LVL_WARNIGN, "unable to download track [%s] cover: %s", track.Id, derr)
				return
			}
			coverType = cType
			coverFile.Sync()
			s, _ := coverFile.Stat()
			buf := make([]byte, s.Size())
			coverFile.Seek(0, io.SeekStart)
			coverFile.Read(buf)
			coverBytes = buf
		}
	}()

	if track.LyricsInfo.HasAvailableSyncLyrics && m.client != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lyr, lerr := m.client.TrackLyricsRequest(track.Id)
			if lerr != nil {
				log.Print(log.LVL_WARNIGN, "failed to obtain track [%s] lyrics: %s", track.Id, lerr)
				m.tracker.ShowError("track lyrics")
				return
			}
			lyrics = lyr
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		tr, ts, cerr := cache.Read(track.Id)
		if cerr == nil {
			trackReader = tr
			trackSize = ts
			trackFromCache = true
			return
		}
		if m.client == nil {
			downloadErr = errors.New("no yandex client")
			return
		}
		var lastErr error
		for i := 0; i < _TRACK_DOWNLOAD_TRIES; i++ {
			trackInfos, ierr := m.client.TrackDownloadInfo(track.Id)
			if ierr != nil {
				log.Print(log.LVL_ERROR, "failed to obtain track [%s] info: %s", track.Id, ierr)
				lastErr = ierr
				continue
			}
			var bestBitrate int
			var bestTrackInfo api.TrackDownloadInfo
			for _, ti := range trackInfos {
				if ti.BbitrateInKbps > bestBitrate {
					bestBitrate = ti.BbitrateInKbps
					bestTrackInfo = ti
				}
			}
			tr2, ts2, derr := m.client.DownloadTrack(bestTrackInfo)
			if derr != nil {
				log.Print(log.LVL_ERROR, "failed to download track [%s]: %s", track.Id, derr)
				lastErr = derr
				continue
			}
			if ts2 <= 0 {
				// an empty body: the track is tried again
				log.Print(log.LVL_ERROR, "track [%s] is empty", track.Id)
				lastErr = errors.New("empty track")
				continue
			}
			trackReader = tr2
			trackSize = ts2
			lastErr = nil
			break
		}
		downloadErr = lastErr
	}()

	wg.Wait()

	if downloadErr != nil && trackReader == nil {
		m.Send(trackReadyMsg{generation: generation, track: track, err: downloadErr})
		return
	}

	trackBuffer := stream.NewBufferedStream(trackReader, trackSize)
	m.writeTrackMetadata(track, trackBuffer, trackFromCache, coverType, coverBytes)
	// the loudness is measured here, in the background, instead of on the
	// audio thread: it decodes ten seconds of audio and used to freeze the
	// window on every track switch
	tracker.MeasureTrack(trackBuffer, track)
	m.Send(trackReadyMsg{
		generation: generation,
		track:      track,
		buffer:     trackBuffer,
		fromCache:  trackFromCache,
		lyrics:     lyrics,
	})
}

// buildTrackTag fills a fresh ID3 tag with the metadata of the track: the name,
// the album, the artists and, when it was downloaded, the cover.
func buildTrackTag(track *api.Track, coverType string, coverBytes []byte) *id3v2.Tag {
	tag := id3v2.NewEmptyTag()
	tag.SetDefaultEncoding(id3v2.EncodingUTF8)
	tag.SetTitle(track.Title)
	if len(track.Albums) != 0 {
		tag.SetAlbum(track.Albums[0].Title)
		tag.SetGenre(track.Albums[0].Genre)
		tag.SetYear(fmt.Sprint(track.Albums[0].Year))
	}
	artists := make([]string, 0, len(track.Artists))
	for i := range track.Artists {
		artists = append(artists, track.Artists[i].Name)
		tag.AddFrame(cache.TAG_ARTIST_ID, id3v2.UserDefinedTextFrame{
			Encoding:    id3v2.EncodingUTF8,
			Description: fmt.Sprint(i),
			Value:       fmt.Sprint(track.Artists[i].Id),
		})
	}
	tag.SetArtist(strings.Join(artists, ","))
	if len(coverBytes) > 0 {
		tag.AddAttachedPicture(id3v2.PictureFrame{
			MimeType:    coverType,
			PictureType: id3v2.PTFrontCover,
			Encoding:    id3v2.EncodingUTF16BE,
			Picture:     coverBytes,
		})
	}
	tag.AddTextFrame(cache.TAG_TRACK_ID, id3v2.EncodingUTF8, fmt.Sprint(track.Id))
	tag.AddTextFrame(cache.TAG_DURATION, id3v2.EncodingUTF8, fmt.Sprint(track.DurationMs))
	return tag
}

// writeTrackMetadata writes the tags and the head of the track to the file the
// media integrations read. A failure is not fatal: the track plays without a
// cover and without scrobbling metadata.
func (m *Model) writeTrackMetadata(track *api.Track, trackBuffer *stream.BufferedStream, fromCache bool, coverType string, coverBytes []byte) {
	metadataFile, err := os.OpenFile(m.metadataFilePath(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		log.Print(log.LVL_WARNIGN, "failed to create metadata file: %s", err)
		return
	}
	defer metadataFile.Close()

	tag := id3v2.NewEmptyTag()
	if fromCache {
		tag.Reset(trackBuffer, id3v2.Options{Parse: true})
	} else {
		tag = buildTrackTag(track, coverType, coverBytes)
	}
	tag.WriteTo(metadataFile)
	io.CopyN(metadataFile, trackBuffer, 32*1024)
	trackBuffer.Seek(0, io.SeekStart)
}

// applyLoadedTrack starts the downloaded track. It runs on the UI loop, so the
// player and the queue are only touched by one goroutine.
func (m *Model) applyLoadedTrack(msg trackReadyMsg) {
	if msg.generation != m.playGeneration {
		// a newer track was requested while this one was downloading
		if msg.buffer != nil {
			msg.buffer.Close()
		}
		return
	}

	m.loadingTrackId = ""
	if msg.err != nil || msg.buffer == nil {
		// the title was already switched to the new track: drop the loading
		// mark so it does not stay "downloading" forever
		m.tracker.Stop()
		m.tracker.ShowError("track download")
		return
	}

	track := msg.track

	if m.currentPlaylistIndex >= 0 && m.client != nil {
		currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
		if currentPlaylist.Rotor {
			ev := api.NewTrackFeedbackEvent(api.EV_TRACK_STARTED, track, 0)
			go m.client.RotorSessionFeedback(currentPlaylist.SessionId, api.NewFeedback(currentPlaylist.SessionBatch, ev))
			log.Print(log.LVL_INFO, "feedback event sended: "+ev.Type+" track: "+track.Title)
		}
	}

	m.tracker.SetNextTrack(m.nextTrackTitle())
	m.tracker.SetRepeatMode(m.repeatMode)
	if !m.tracker.StartTrack(track, msg.buffer, msg.lyrics) {
		// the track is broken: the next one is tried instead
		m.failedTracks++
		if m.failedTracks > _TRACK_FAILURES_LIMIT {
			m.failedTracks = 0
			m.Send(tracker.STOP)
			return
		}
		m.nextTrack()
		return
	}
	m.failedTracks = 0
	m.scrobbleStart(track)
	m.discordUpdate(track, true)
	m.indicateCurrentTrackPlaying(true)
	m.mediaHandler.OnPlayback()

	if m.client != nil {
		go m.client.PlayTrack(track, msg.fromCache)
	}
}

// nextTrackTitle returns a human readable name of the track that will be
// played next, or an empty string when the queue is over.
func (m *Model) nextTrackTitle() string {
	if len(m.manualQueue) > 0 {
		track := &m.manualQueue[0]
		return helpers.ArtistList(track.Artists) + " — " + track.Title
	}
	for i := m.playQueueTrackIndex + 1; i < len(m.playQueue); i++ {
		track := &m.playQueue[i]
		if !track.Available {
			continue
		}
		return helpers.ArtistList(track.Artists) + " — " + track.Title
	}
	return ""
}

func (m *Model) playSelectedPlaylist(trackIndex int) {
	selectedPlaylist := m.playlists.SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 || selectedPlaylist.SelectedTrack < 0 || selectedPlaylist.SelectedTrack >= len(selectedPlaylist.Tracks) {
		m.Send(tracker.STOP)
		return
	}

	trackToPlay := &selectedPlaylist.Tracks[selectedPlaylist.SelectedTrack]

	// clicking the track that is still downloading must not start a second
	// download: there is nothing to pause or resume yet
	if m.loadingTrackId == trackToPlay.Id && m.tracker.IsStoped() {
		return
	}

	if m.currentPlaylistIndex >= 0 {
		currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
		if currentPlaylist.IsSame(selectedPlaylist) && selectedPlaylist.CurrentTrack == trackIndex && m.tracker.CurrentTrack().Id == trackToPlay.Id {
			if m.tracker.IsPlaying() {
				m.tracker.Pause()
				return
			} else {
				m.tracker.Play()
				return
			}
		}
		if currentPlaylist.Rotor {
			if m.tracker.IsPlaying() {
				go m.client.RotorSessionFeedback(currentPlaylist.SessionId, m.feedbackOnTrack(currentPlaylist.SessionBatch))
			}
			if !currentPlaylist.IsSame(selectedPlaylist) {
				ev := api.NewRadioFeedbackEvent(api.EV_RADIO_FINISHED)
				go m.client.RotorSessionFeedback(currentPlaylist.SessionId, api.NewFeedback(currentPlaylist.SessionBatch, ev))
				log.Print(log.LVL_INFO, "feedback event sended: "+ev.Type)
			}
		}
	}

	m.indicateCurrentTrackPlaying(false)
	// a playlist track is chosen right away: the manual one is over
	m.manualPlaying = nil

	if selectedPlaylist.Rotor {
		if m.currentPlaylistIndex != m.playlists.Index() {
			ev := api.NewRadioFeedbackEvent(api.EV_RADIO_STARTED)
			go m.client.RotorSessionFeedback(selectedPlaylist.SessionId, api.NewFeedback("", ev))
			log.Print(log.LVL_INFO, "feedback event sended: "+ev.Type)
			m.currentPlaylistIndex = m.playlists.Index()
		}
		if trackIndex == len(selectedPlaylist.Tracks)-1 {
			m.rotateTracks(selectedPlaylist)
		}
	}

	if len(selectedPlaylist.Albums) > 0 && selectedPlaylist.SelectedAlbum >= 0 {
		m.currentAlbumIndex = selectedPlaylist.SelectedAlbum
	} else {
		m.currentAlbumIndex = -1
	}

	selectedPlaylist.CurrentTrack = trackIndex
	m.currentPlaylistIndex = m.playlists.Index()
	m.playlists.SetItem(m.currentPlaylistIndex, selectedPlaylist)
	m.playQueue = selectedPlaylist.Tracks
	m.playQueueTrackIndex = trackIndex
	m.playTrack(trackToPlay)
}

// repeatWrapIndex tells where the playback goes after the last track of the
// queue: the track that just played (RepeatOne), the first one (RepeatAll) or
// nowhere (RepeatOff, -1).
func repeatWrapIndex(queueLen int, mode tracker.RepeatMode) int {
	switch mode {
	case tracker.RepeatOne:
		return queueLen - 1
	case tracker.RepeatAll:
		return 0
	default:
		return -1
	}
}

// toggleRepeat cycles through repeat modes: off → all → one → off.
func (m *Model) toggleRepeat() {
	switch m.repeatMode {
	case tracker.RepeatOff:
		m.repeatMode = tracker.RepeatAll
	case tracker.RepeatAll:
		m.repeatMode = tracker.RepeatOne
	default:
		m.repeatMode = tracker.RepeatOff
	}
	m.showToast("Повтор: " + repeatLabel(m.repeatMode))
}

// repeatLabel names a repeat mode for the notification.
func repeatLabel(mode tracker.RepeatMode) string {
	switch mode {
	case tracker.RepeatAll:
		return "весь плейлист"
	case tracker.RepeatOne:
		return "один трек"
	default:
		return "выкл"
	}
}
