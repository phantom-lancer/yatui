package mainpage

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
)

type rotorFeedbackResultMsg struct {
	eventType string
	err       error
}

type trackLikeResultMsg struct {
	track         api.Track
	liked         bool
	sessionID     string
	batchID       string
	rotorFeedback bool
	err           error
}

func trackLikeCommand(change func(string, bool) error, track api.Track, liked bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if change == nil {
			err = fmt.Errorf("track Likes request is unavailable")
		} else {
			err = change(track.Id, liked)
		}
		return trackLikeResultMsg{track: track, liked: liked, err: err}
	}
}

func applyLikedState(likedTracks map[string]bool, trackID string, liked bool, err error) bool {
	if err != nil || likedTracks == nil || trackID == "" {
		return false
	}
	if liked {
		likedTracks[trackID] = true
	} else {
		delete(likedTracks, trackID)
	}
	return true
}

func rotorFeedbackCmd(send func(string, *api.RotorFeedback) error, sessionID, batchID string, event *api.RotorFeedbackEvent) tea.Cmd {
	return func() tea.Msg {
		if send == nil || event == nil {
			return rotorFeedbackResultMsg{eventType: eventType(event), err: fmt.Errorf("rotor feedback sender is unavailable")}
		}
		return rotorFeedbackResultMsg{
			eventType: event.Type,
			err:       send(sessionID, api.NewFeedback(batchID, event)),
		}
	}
}

func eventType(event *api.RotorFeedbackEvent) string {
	if event == nil {
		return "unknown"
	}
	return event.Type
}

func shouldSendNextTrackFeedback(enabled, rotor, playing bool) bool {
	return enabled && rotor && playing
}

func (m *Model) likePlayingTrack() tea.Cmd {
	var currentPlaylist *playlist.Item
	if m.currentPlaylistIndex >= 0 {
		currentPlaylist = m.playlists.Items()[m.currentPlaylistIndex]
	}

	track := m.tracker.CurrentTrack()
	return m.likeTrack(track, currentPlaylist)
}

func (m *Model) likeSelectedTrack() tea.Cmd {
	if m.currentPlaylistIndex < 0 {
		return nil
	}

	selectedPlaylist := m.playlists.SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 {
		return nil
	}

	track := m.tracklist.SelectedItem().Track
	return m.likeTrack(track, selectedPlaylist)
}

// dislikePlayingTrack removes the like (if any) from the playing track
// and, while listening to "my wave", skips to the next track.
func (m *Model) dislikePlayingTrack() tea.Cmd {
	if m.tracker.IsStoped() {
		return nil
	}

	var currentPlaylist *playlist.Item
	if m.currentPlaylistIndex >= 0 {
		currentPlaylist = m.playlists.Items()[m.currentPlaylistIndex]
	}

	track := m.tracker.CurrentTrack()
	if track == nil {
		return nil
	}

	// The dislike is a Rotor event, not a local-only mark or a generic skip.
	// It is sent before advancing, with the actual listening time.
	m.dislikedTracksMap[track.Id] = true

	if currentPlaylist != nil && currentPlaylist.Rotor {
		var cmds []tea.Cmd
		if m.client != nil {
			event := api.NewTrackFeedbackEvent(api.EV_TRACK_DISLIKED, track, m.tracker.Playtime().Seconds())
			cmds = append(cmds, rotorFeedbackCmd(m.client.RotorSessionFeedback, currentPlaylist.SessionId, currentPlaylist.SessionBatch, event))
		}

		if m.likedTracksMap[track.Id] {
			cmds = append(cmds, m.requestTrackLike(track, false, nil, false))
		}

		// The dislike event already describes this transition. Do not follow it
		// with the normal skip/finished event from nextTrack.
		m.nextTrackWithFeedback(false)
		return tea.Batch(cmds...)
	}

	if m.likedTracksMap[track.Id] {
		return m.requestTrackLike(track, false, nil, false)
	}

	return nil
}

// dislikeTrackAt dislikes the track at the given index of the tracks list.
func (m *Model) dislikeTrackAt(index int) tea.Cmd {
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

	track := items[index].Track
	m.dislikedTracksMap[track.Id] = true

	if selectedPlaylist.Rotor {
		if m.tracker.CurrentTrack() != nil && m.tracker.CurrentTrack().Id == track.Id {
			return m.dislikePlayingTrack()
		}

		if m.likedTracksMap[track.Id] {
			return m.requestTrackLike(track, false, nil, false)
		}
		return nil
	}

	if m.likedTracksMap[track.Id] {
		return m.requestTrackLike(track, false, nil, false)
	}

	return nil
}

func (m *Model) likeTrack(track *api.Track, pl *playlist.Item) tea.Cmd {
	if track == nil {
		return nil
	}
	return m.requestTrackLike(track, !m.likedTracksMap[track.Id], pl, true)
}

// requestTrackLike schedules a collection mutation without blocking the UI.
// The local state is changed only by applyTrackLikeResult after success.
func (m *Model) requestTrackLike(track *api.Track, liked bool, pl *playlist.Item, sendRotorFeedback bool) tea.Cmd {
	if track == nil || m.client == nil || m.likedTracksMap[track.Id] == liked {
		return nil
	}
	if m.pendingLikeTracks == nil {
		m.pendingLikeTracks = make(map[string]bool)
	}
	if m.pendingLikeTracks[track.Id] {
		return nil
	}
	m.pendingLikeTracks[track.Id] = true

	client := m.client
	sessionID, batchID := "", ""
	if sendRotorFeedback && pl != nil && pl.Rotor {
		sessionID, batchID = pl.SessionId, pl.SessionBatch
	}

	var change = func(trackID string, wantLiked bool) error {
		if wantLiked {
			return client.LikeTrack(trackID)
		}
		return client.UnlikeTrack(trackID)
	}
	request := trackLikeCommand(change, *track, liked)

	return func() tea.Msg {
		result := request().(trackLikeResultMsg)
		result.sessionID = sessionID
		result.batchID = batchID
		result.rotorFeedback = sessionID != ""
		return result
	}
}

func (m *Model) applyTrackLikeResult(result trackLikeResultMsg) tea.Cmd {
	if m.pendingLikeTracks != nil {
		delete(m.pendingLikeTracks, result.track.Id)
	}
	if result.err != nil {
		log.Print(log.LVL_ERROR, "failed to update Likes for track %s: %s", result.track.Id, result.err)
		m.showToast("Не удалось обновить лайки: " + result.track.Title)
		return nil
	}

	if m.likedTracksMap == nil {
		m.likedTracksMap = make(map[string]bool)
	}
	applyLikedState(m.likedTracksMap, result.track.Id, result.liked, nil)

	var cmds []tea.Cmd
	if m.playlists != nil {
		likedPlaylist, index := m.playlists.GetFirst(playlist.LIKES)
		if likedPlaylist != nil && index >= 0 {
			if result.liked {
				likedPlaylist.AddTrack(&result.track)
			} else {
				likedPlaylist.RemoveTrack(result.track.Id)
			}
			cmds = append(cmds, m.playlists.SetItem(index, likedPlaylist))
			items := m.playlists.Items()
			if len(items) > 0 && m.playlists.SelectedItem().Kind == playlist.LIKES {
				m.displayPlaylist(likedPlaylist)
			}
		}
	}

	if result.rotorFeedback && m.client != nil {
		eventType := api.EV_TRACK_UNLIKED
		if result.liked {
			eventType = api.EV_TRACK_LIKED
		}
		event := api.NewTrackFeedbackEvent(eventType, &result.track, 0)
		cmds = append(cmds, rotorFeedbackCmd(m.client.RotorSessionFeedback, result.sessionID, result.batchID, event))
	}

	if result.liked {
		m.showToast("В лайки: " + result.track.Title)
	} else {
		m.showToast("Убрано из лайков: " + result.track.Title)
	}
	if m.tracker != nil {
		m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
	}
	return tea.Batch(cmds...)
}
