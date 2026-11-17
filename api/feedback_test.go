package api

import (
	"encoding/json"
	"testing"
)

func TestNewTrackDislikeFeedbackIncludesRotorFields(t *testing.T) {
	track := &Track{
		Id:         "track-123",
		DurationMs: 245500,
		Albums:     []Album{{Id: 456}},
	}

	event := NewTrackFeedbackEvent(EV_TRACK_DISLIKED, track, 17.5)
	payload, err := json.Marshal(NewFeedback("batch-789", event))
	if err != nil {
		t.Fatalf("marshal feedback: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal feedback: %v", err)
	}
	feedbackEvent, ok := got["event"].(map[string]any)
	if !ok {
		t.Fatalf("event field has type %T", got["event"])
	}

	if feedbackEvent["type"] != "dislike" {
		t.Errorf("event type = %v, want dislike", feedbackEvent["type"])
	}
	if feedbackEvent["trackId"] != "track-123:456" {
		t.Errorf("track ID = %v, want album-qualified track ID", feedbackEvent["trackId"])
	}
	if feedbackEvent["totalPlayedSeconds"] != 17.5 {
		t.Errorf("played seconds = %v, want 17.5", feedbackEvent["totalPlayedSeconds"])
	}
	if feedbackEvent["trackLengthSeconds"] != 245.5 {
		t.Errorf("track length seconds = %v, want 245.5", feedbackEvent["trackLengthSeconds"])
	}
	if feedbackEvent["timestamp"] == "" || feedbackEvent["timestamp"] == nil {
		t.Error("timestamp is empty")
	}
	if got["from"] == "" || got["batchId"] != "batch-789" {
		t.Errorf("outer feedback metadata: from=%v batch=%v", got["from"], got["batchId"])
	}
}
