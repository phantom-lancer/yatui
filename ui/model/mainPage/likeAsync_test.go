package mainpage

import (
	"errors"
	"testing"

	"github.com/phantom-lancer/yatui/api"
)

func TestTrackLikeCommandRunsRequestAndReturnsResult(t *testing.T) {
	calls := 0
	change := func(trackID string, liked bool) error {
		calls++
		if trackID != "track-1" || !liked {
			t.Fatalf("request = (%q, %v), want (track-1, true)", trackID, liked)
		}
		return nil
	}
	track := api.Track{Id: "track-1", Title: "Track"}
	msg := trackLikeCommand(change, track, true)().(trackLikeResultMsg)

	if calls != 1 {
		t.Fatalf("request called %d times, want 1", calls)
	}
	if msg.track.Id != track.Id || !msg.liked || msg.err != nil {
		t.Fatalf("unexpected result: %+v", msg)
	}
}

func TestApplyLikedStateChangesOnlyAfterSuccessfulRequest(t *testing.T) {
	liked := map[string]bool{"track-1": false}

	if applyLikedState(liked, "track-1", true, errors.New("offline")) {
		t.Fatal("failed request must not be applied")
	}
	if liked["track-1"] {
		t.Fatal("failed request changed local Likes state")
	}

	if !applyLikedState(liked, "track-1", true, nil) || !liked["track-1"] {
		t.Fatal("successful like was not applied")
	}
	if !applyLikedState(liked, "track-1", false, nil) || liked["track-1"] {
		t.Fatal("successful unlike was not applied")
	}
}
