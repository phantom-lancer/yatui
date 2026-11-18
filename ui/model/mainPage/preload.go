package mainpage

import (
	"sync"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/cache"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/stream"
	"github.com/phantom-lancer/yatui/ui/components/tracker"
)

const preloadMaxTrackBytes int64 = 20 << 20

type preloadedTrack struct {
	id     string
	buffer *stream.BufferedStream
}

type preloadState struct {
	mu         sync.Mutex
	generation uint64
	item       *preloadedTrack
}

// takePreloaded transfers ownership of a ready next-track buffer to loadTrack.
// It also invalidates any older in-flight prefetch so it cannot publish stale
// audio after the queue has changed.
func (m *Model) takePreloaded(trackID string) *stream.BufferedStream {
	m.preload.mu.Lock()
	m.preload.generation++
	item := m.preload.item
	m.preload.item = nil
	m.preload.mu.Unlock()

	if item == nil {
		return nil
	}
	if item.id == trackID {
		return item.buffer
	}
	_ = item.buffer.Close()
	return nil
}

func (m *Model) nextTrackForPreload() *api.Track {
	for _, track := range m.manualQueue {
		if track.Available {
			return &track
		}
	}
	for i := m.playQueueTrackIndex + 1; i < len(m.playQueue); i++ {
		if m.playQueue[i].Available {
			return &m.playQueue[i]
		}
	}
	if m.repeatMode == tracker.RepeatAll {
		for i := 0; i < len(m.playQueue); i++ {
			if m.playQueue[i].Available {
				return &m.playQueue[i]
			}
		}
	}
	return nil
}

// preloadNextTrack downloads only the next playable track, with a strict
// 20 MiB cap. One bounded BufferedStream is retained; no playlist is queued
// into RAM and the user's normal cache policy is not changed.
func (m *Model) clearPreload() {
	m.preload.mu.Lock()
	m.preload.generation++
	old := m.preload.item
	m.preload.item = nil
	m.preload.mu.Unlock()
	if old != nil {
		_ = old.buffer.Close()
	}
}

func (m *Model) preloadNextTrack() {
	if !config.Current.PreloadNextTrack {
		m.clearPreload()
		return
	}
	// A new playback state invalidates the previous prediction, including a
	// prefetch that has not finished publishing its buffer yet.
	m.preload.mu.Lock()
	m.preload.generation++
	generation := m.preload.generation
	old := m.preload.item
	m.preload.item = nil
	m.preload.mu.Unlock()
	if old != nil {
		_ = old.buffer.Close()
	}

	if m.client == nil {
		return
	}
	track := m.nextTrackForPreload()
	if track == nil || track.Id == "" || cache.Exists(track.Id) {
		return
	}
	if current := m.tracker.CurrentTrack(); current != nil && current.Id == track.Id {
		return
	}

	trackCopy := *track
	go func() {
		infos, err := m.client.TrackDownloadInfo(trackCopy.Id)
		if err != nil {
			log.Print(log.LVL_INFO, "next track prefetch skipped: %s", err)
			return
		}

		var bestInfo api.TrackDownloadInfo
		bestBitrate := 0
		for _, info := range infos {
			if info.BbitrateInKbps > bestBitrate {
				bestBitrate = info.BbitrateInKbps
				bestInfo = info
			}
		}
		if bestBitrate == 0 {
			return
		}

		reader, size, err := m.client.DownloadTrack(bestInfo)
		if err != nil {
			log.Print(log.LVL_INFO, "next track prefetch skipped: %s", err)
			return
		}
		if size <= 0 || size > preloadMaxTrackBytes {
			reader.Close()
			return
		}

		buffer := stream.NewBufferedStream(reader, size)
		m.preload.mu.Lock()
		if generation != m.preload.generation {
			m.preload.mu.Unlock()
			_ = buffer.Close()
			return
		}
		m.preload.item = &preloadedTrack{id: trackCopy.Id, buffer: buffer}
		m.preload.mu.Unlock()
		log.Print(log.LVL_INFO, "next track prefetched: %s (%d bytes)", trackCopy.Title, size)
	}()
}
