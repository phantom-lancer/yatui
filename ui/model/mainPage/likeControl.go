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

		if m.likedTracksMap[track.Id] && m.client != nil {
			if err := m.client.UnlikeTrack(track.Id); err == nil {
				delete(m.likedTracksMap, track.Id)
				likedPlaylist, index := m.playlists.GetFirst(playlist.LIKES)
				likedPlaylist.RemoveTrack(track.Id)
				updateCmd := m.playlists.SetItem(index, likedPlaylist)
				if m.playlists.SelectedItem().Kind == playlist.LIKES {
					m.displayPlaylist(likedPlaylist)
				}
				cmds = append(cmds, updateCmd)
			} else {
				log.Print(log.LVL_ERROR, "failed to remove disliked track from Likes: %s", err)
			}
		}

		// The dislike event already describes this transition. Do not follow it
		// with the normal skip/finished event from nextTrack.
		m.nextTrackWithFeedback(false)
		return tea.Batch(cmds...)
	}

	if m.likedTracksMap[track.Id] {
		m.client.UnlikeTrack(track.Id)
		delete(m.likedTracksMap, track.Id)

		likedPlaylist, index := m.playlists.GetFirst(playlist.LIKES)
		likedPlaylist.RemoveTrack(track.Id)
		cmd := m.playlists.SetItem(index, likedPlaylist)
		if m.playlists.SelectedItem().Kind == playlist.LIKES {
			m.displayPlaylist(likedPlaylist)
		}
		m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
		return cmd
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
			m.client.UnlikeTrack(track.Id)
			delete(m.likedTracksMap, track.Id)

			likedPlaylist, idx := m.playlists.GetFirst(playlist.LIKES)
			likedPlaylist.RemoveTrack(track.Id)
			cmd := m.playlists.SetItem(idx, likedPlaylist)
			if m.playlists.SelectedItem().Kind == playlist.LIKES {
				m.displayPlaylist(likedPlaylist)
			}
			m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
			return cmd
		}
		return nil
	}

	if m.likedTracksMap[track.Id] {
		m.client.UnlikeTrack(track.Id)
		delete(m.likedTracksMap, track.Id)

		likedPlaylist, idx := m.playlists.GetFirst(playlist.LIKES)
		likedPlaylist.RemoveTrack(track.Id)
		cmd := m.playlists.SetItem(idx, likedPlaylist)
		if m.playlists.SelectedItem().Kind == playlist.LIKES {
			m.displayPlaylist(likedPlaylist)
		}
		m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
		return cmd
	}

	return nil
}

func (m *Model) likeTrack(track *api.Track, pl *playlist.Item) tea.Cmd {
	likedPlaylist, index := m.playlists.GetFirst(playlist.LIKES)

	var evType api.TrackEventType
	if m.likedTracksMap[track.Id] {
		if m.client.UnlikeTrack(track.Id) != nil {
			return nil
		}
		delete(m.likedTracksMap, track.Id)
		likedPlaylist.RemoveTrack(track.Id)
		evType = api.EV_TRACK_UNLIKED
	} else {
		if m.client.LikeTrack(track.Id) != nil {
			return nil
		}
		m.likedTracksMap[track.Id] = true
		likedPlaylist.AddTrack(track)
		evType = api.EV_TRACK_LIKED
	}

	if pl != nil && pl.Rotor {
		ev := api.NewTrackFeedbackEvent(evType, track, 0)
		go m.client.RotorSessionFeedback(pl.SessionId, api.NewFeedback(pl.SessionBatch, ev))
		log.Print(log.LVL_INFO, "feedback event sended: "+ev.Type+" track: "+track.Title)
	}

	cmd := m.playlists.SetItem(index, likedPlaylist)
	if m.playlists.SelectedItem().Kind == playlist.LIKES {
		m.displayPlaylist(likedPlaylist)
	}

	if evType == api.EV_TRACK_LIKED {
		m.showToast("В лайки: " + track.Title)
	} else {
		m.showToast("Убрано из лайков: " + track.Title)
	}

	m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
	return cmd
}
