package mainpage

import (
	"testing"

	"github.com/phantom-lancer/yatui/api"
)

func TestDislikeInOrdinaryPlaylistIsVisibleLocally(t *testing.T) {
	m := &Model{currentPlaylistIndex: 0}
	m.playlists = testPlaylists(t, []string{"playlist"})
	selected := m.playlists.SelectedItem()
	selected.Tracks = []api.Track{testTrack("1", "Track One")}
	m.playlists.SetItem(0, selected)
	m.tracklist = testTracklist(t, []string{"Track One"})

	m.dislikeTrackAt(0)

	if !m.dislikedTracksMap["1"] {
		t.Fatal("ordinary playlist dislike was not recorded locally")
	}
	if len(m.toasts) == 0 || m.toasts[len(m.toasts)-1].text != "Дизлайк локально: Track One" {
		t.Fatalf("missing honest dislike notice: %+v", m.toasts)
	}
}

func TestNextTrackFeedbackCanBeSuppressed(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		rotor    bool
		playing  bool
		wantSend bool
	}{
		{name: "ordinary radio skip", enabled: true, rotor: true, playing: true, wantSend: true},
		{name: "non-radio track", enabled: true, rotor: false, playing: true, wantSend: false},
		{name: "paused radio", enabled: true, rotor: true, playing: false, wantSend: false},
		{name: "dislike already reported", enabled: false, rotor: true, playing: true, wantSend: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSendNextTrackFeedback(tt.enabled, tt.rotor, tt.playing); got != tt.wantSend {
				t.Fatalf("shouldSendNextTrackFeedback() = %v, want %v", got, tt.wantSend)
			}
		})
	}
}

func TestRotorFeedbackCommandSendsOneDislikeEvent(t *testing.T) {
	var calls int
	var gotSession string
	var gotFeedback *api.RotorFeedback
	send := func(session string, feedback *api.RotorFeedback) error {
		calls++
		gotSession = session
		gotFeedback = feedback
		return nil
	}
	event := api.NewTrackFeedbackEvent(api.EV_TRACK_DISLIKED,
		&api.Track{Id: "42", DurationMs: 180000}, 12.25)

	msg := rotorFeedbackCmd(send, "session-1", "batch-2", event)().(rotorFeedbackResultMsg)
	if msg.err != nil {
		t.Fatalf("unexpected feedback error: %v", msg.err)
	}
	if calls != 1 {
		t.Fatalf("feedback sent %d times, want 1", calls)
	}
	if gotSession != "session-1" || gotFeedback == nil {
		t.Fatalf("session=%q feedback=%+v", gotSession, gotFeedback)
	}
	if gotFeedback.BatchId != "batch-2" || gotFeedback.Event.Type != "dislike" {
		t.Fatalf("wrong feedback payload: %+v", gotFeedback)
	}
	if gotFeedback.Event.TotalPlayedSeconds != 12.25 {
		t.Fatalf("played seconds = %v, want 12.25", gotFeedback.Event.TotalPlayedSeconds)
	}
}
