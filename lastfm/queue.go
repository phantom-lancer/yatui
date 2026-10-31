package lastfm

import (
	"encoding/json"
	"os"
	"sync"
)

// maxQueuedScrobbles caps the file: a very long run without network must not
// grow it without limit. The oldest reports are dropped first.
const maxQueuedScrobbles = 1000

// Scrobble is one track report waiting to be delivered to Last.fm.
type Scrobble struct {
	Artist    string `json:"artist"`
	Title     string `json:"title"`
	Album     string `json:"album,omitempty"`
	Timestamp int64  `json:"timestamp"`
	Duration  int    `json:"duration,omitempty"`
}

// Queue is the persistent list of the reports Last.fm did not take: they are
// kept across restarts and sent when the connection is back. An empty path
// keeps the queue in memory, which is what the tests use.
type Queue struct {
	path  string
	mu    sync.Mutex
	items []Scrobble
}

// NewQueue reads the queue from the file. A missing or broken file is an empty
// queue: a lost report must not stop the application.
func NewQueue(path string) *Queue {
	q := &Queue{path: path}
	if path == "" {
		return q
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return q
	}
	_ = json.Unmarshal(data, &q.items)
	if len(q.items) > maxQueuedScrobbles {
		q.items = q.items[len(q.items)-maxQueuedScrobbles:]
	}
	return q
}

// Len reports how many reports are waiting.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Add appends a report and saves the queue. It is called off the UI thread.
func (q *Queue) Add(s Scrobble) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.items = append(q.items, s)
	if len(q.items) > maxQueuedScrobbles {
		q.items = q.items[len(q.items)-maxQueuedScrobbles:]
	}
	q.saveLocked()
}

// Flush sends the queued reports in order. A report Last.fm will never accept
// is dropped, and the first one that failed because of the network stops the
// flush: the rest waits for the next try.
func (q *Queue) Flush(send func(Scrobble) error) (sent int) {
	for {
		q.mu.Lock()
		if len(q.items) == 0 {
			q.mu.Unlock()
			return sent
		}
		item := q.items[0]
		q.mu.Unlock()

		err := send(item)

		q.mu.Lock()
		if err != nil && !Permanent(err) {
			// the connection is down: keep this one and everything after it
			q.mu.Unlock()
			return sent
		}
		q.items = q.items[1:]
		q.saveLocked()
		q.mu.Unlock()

		if err == nil {
			sent++
		}
	}
}

// saveLocked writes the queue to the disk. The write is atomic: a crash leaves
// either the old file or the new one, never half of the list.
func (q *Queue) saveLocked() {
	if q.path == "" {
		return
	}
	if len(q.items) == 0 {
		_ = os.Remove(q.path)
		return
	}

	data, err := json.Marshal(q.items)
	if err != nil {
		return
	}
	tmp := q.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return
	}
	_ = os.Rename(tmp, q.path)
}
