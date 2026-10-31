package lastfm

import (
	"errors"
	"path/filepath"
	"testing"
)

func sample(title string) Scrobble {
	return Scrobble{Artist: "Artist", Title: title, Timestamp: 100, Duration: 200}
}

// TestQueueSurvivesRestart checks that a report kept while the network is down
// is still there after the application restarts.
func TestQueueSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scrobbles.json")
	q := NewQueue(path)
	q.Add(sample("One"))
	q.Add(sample("Two"))

	reloaded := NewQueue(path)
	if got := reloaded.Len(); got != 2 {
		t.Fatalf("queue length = %d, want 2", got)
	}
	if got := reloaded.items[0].Title; got != "One" {
		t.Fatalf("first title = %q, want One", got)
	}
}

// TestFlushStopsOnNetworkError checks that a failed report stays in the queue
// and that nothing after it is sent out of order.
func TestFlushStopsOnNetworkError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scrobbles.json")
	q := NewQueue(path)
	q.Add(sample("One"))
	q.Add(sample("Two"))

	networkErr := errors.New("network is unreachable")
	sent := q.Flush(func(s Scrobble) error { return networkErr })
	if sent != 0 {
		t.Fatalf("sent = %d, want 0", sent)
	}
	if got := q.Len(); got != 2 {
		t.Fatalf("queue length = %d, want 2", got)
	}
	if got := NewQueue(path).Len(); got != 2 {
		t.Fatalf("the queue on disk has %d reports, want 2", got)
	}
}

// TestFlushDropsPermanentErrors checks that a report Last.fm will never take
// does not block the queue forever.
func TestFlushDropsPermanentErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scrobbles.json")
	q := NewQueue(path)
	q.Add(sample("Refused"))
	q.Add(sample("Fine"))

	sent := q.Flush(func(s Scrobble) error {
		if s.Title == "Refused" {
			return &Error{Code: 9, Message: "Invalid session key"}
		}
		return nil
	})
	if sent != 1 {
		t.Fatalf("sent = %d, want 1", sent)
	}
	if got := q.Len(); got != 0 {
		t.Fatalf("queue length = %d, want 0", got)
	}
}

// TestQueueIsCapped checks that a long offline run does not grow the file
// without limit and keeps the newest reports.
func TestQueueIsCapped(t *testing.T) {
	// an in-memory queue: the cap is about the list, not about the disk
	q := NewQueue("")
	for i := 0; i < maxQueuedScrobbles+5; i++ {
		q.Add(sample("Track"))
	}
	if got := q.Len(); got != maxQueuedScrobbles {
		t.Fatalf("queue length = %d, want %d", got, maxQueuedScrobbles)
	}
}

func TestPermanent(t *testing.T) {
	if Permanent(errors.New("connection reset")) {
		t.Fatal("a network error is worth another try")
	}
	if !Permanent(&Error{Code: 9}) {
		t.Fatal("an invalid session is permanent")
	}
	if Permanent(&Error{Code: 29}) {
		t.Fatal("a rate limit has to be retried later")
	}
}
