package mainpage

import (
	"fmt"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/dece2183/yamusic-tui/ui/helpers"
	"github.com/dece2183/yamusic-tui/ui/style"
)

// _TOAST_TITLE_LIMIT keeps a toast to a single short line.
const _TOAST_TITLE_LIMIT = 46

// showPlayStateToast reports that the playback was paused or resumed.
func (m *Model) showPlayStateToast() {
	if m.tracker.Paused() {
		m.showToastKind("playstate", "⏸ Пауза")
	} else {
		m.showToastKind("playstate", style.IconPlay+" Играет")
	}
}

// showPlayingToast names the track that just started.
func (m *Model) showPlayingToast() {
	track := m.tracker.CurrentTrack()
	if track == nil || track.Id == "" || !m.tracker.IsPlaying() {
		return
	}

	title := track.Title
	if artists := helpers.ArtistList(track.Artists); artists != "" {
		title = artists + " — " + title
	}

	m.showToastKind("track", style.IconPlay+" "+ansi.Truncate(title, _TOAST_TITLE_LIMIT, "…"))
}

// showVolumeToast reports the new volume. The messages replace each other, so
// holding the key down leaves a single line in the corner.
func (m *Model) showVolumeToast() {
	volume := m.tracker.Volume()

	icon := style.IconVolumeHigh
	switch {
	case volume <= 0:
		icon = style.IconVolumeOff
	case volume < 0.1:
		icon = style.IconVolumeLow
	case volume < 0.5:
		icon = style.IconVolumeMid
	}

	m.showToastKind("volume", fmt.Sprintf("%s Громкость: %d%%", icon, int(volume*100+0.5)))
}

// showSeekToast reports where the playback was moved to.
func (m *Model) showSeekToast() {
	track := m.tracker.CurrentTrack()
	if track == nil {
		return
	}

	m.showToastKind("seek", fmt.Sprintf("⏩ %s / %s",
		formatPlaytime(m.tracker.Position()),
		formatPlaytime(time.Duration(track.DurationMs)*time.Millisecond)))
}

// showPlaylistToast reports which playlist the track list belongs to.
func (m *Model) showPlaylistToast(name string) {
	if name == "" {
		return
	}
	m.showToastKind("playlist", "Плейлист: "+ansi.Truncate(name, _TOAST_TITLE_LIMIT, "…"))
}

// formatPlaytime renders a duration as m:ss (or h:mm:ss for long ones).
func formatPlaytime(d time.Duration) string {
	total := int(d.Seconds())
	if total < 0 {
		total = 0
	}

	hours := total / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}
