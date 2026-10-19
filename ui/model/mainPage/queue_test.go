package mainpage

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/dece2183/yamusic-tui/api"
)

func testTrack(id, title string) api.Track {
	track := api.Track{Id: id, Title: title, Available: true, DurationMs: 180000}
	track.Artists = []api.Artist{{Name: "artist"}}
	return track
}

func TestAddToQueue(t *testing.T) {
	m := &Model{}

	m.addNext(&api.Track{Id: "a", Title: "A"})
	m.addNext(&api.Track{Id: "b", Title: "B"})
	m.addToQueue(&api.Track{Id: "c", Title: "C"})

	if len(m.manualQueue) != 3 {
		t.Fatalf("queue length: %d", len(m.manualQueue))
	}
	// addNext puts tracks in front, so the last added goes first
	if m.manualQueue[0].Id != "b" || m.manualQueue[1].Id != "a" || m.manualQueue[2].Id != "c" {
		t.Fatalf("unexpected order: %v", m.manualQueue)
	}

	m.addNext(nil)
	m.addToQueue(nil)
	if len(m.manualQueue) != 3 {
		t.Fatal("nil tracks must be ignored")
	}
}

func TestQueueEntriesPutManualFirst(t *testing.T) {
	m := &Model{}
	m.manualQueue = []api.Track{testTrack("m1", "Manual")}
	m.playQueue = []api.Track{
		testTrack("p0", "Current"),
		testTrack("p1", "Next"),
		testTrack("p2", "Later"),
	}
	m.playQueueTrackIndex = 0

	entries := m.queueEntries()
	if len(entries) != 3 {
		t.Fatalf("entries: %d", len(entries))
	}
	if !entries[0].manual || entries[0].track.Id != "m1" {
		t.Fatalf("first entry must be the manual one: %+v", entries[0])
	}
	if entries[1].manual || entries[1].track.Id != "p1" {
		t.Fatalf("second entry must be the playlist one: %+v", entries[1])
	}
	if entries[2].track.Id != "p2" {
		t.Fatalf("third entry must be p2: %+v", entries[2])
	}
}

func TestNextTrackTitlePrefersQueue(t *testing.T) {
	m := &Model{}
	m.playQueue = []api.Track{testTrack("p0", "Current"), testTrack("p1", "Playlist")}
	m.playQueueTrackIndex = 0

	if got := m.nextTrackTitle(); got != "artist — Playlist" {
		t.Fatalf("playlist next title: %q", got)
	}

	m.addNext(&api.Track{Id: "m", Title: "Queued", Available: true})
	if got := m.nextTrackTitle(); !strings.Contains(got, "Queued") {
		t.Fatalf("queue must win, got %q", got)
	}
}

func TestQueueScrollKeepsSelectionVisible(t *testing.T) {
	q := &queueModel{index: 9}
	q.ensureVisible(20, 5)
	if q.offset != 5 {
		t.Fatalf("offset: %d", q.offset)
	}

	q.index = 2
	q.ensureVisible(20, 5)
	if q.offset != 2 {
		t.Fatalf("offset must follow the selection up: %d", q.offset)
	}

	// a short queue never scrolls
	q.index = 1
	q.ensureVisible(3, 5)
	if q.offset != 0 {
		t.Fatalf("offset for a short queue: %d", q.offset)
	}
}

func TestQueueViewDoesNotPanic(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.manualQueue = []api.Track{testTrack("m1", "Queued track")}
	m.playQueue = []api.Track{
		testTrack("p0", "Current"),
		testTrack("p1", "A rather long next track title that has to be shortened"),
	}
	m.playQueueTrackIndex = 0
	m.queue = &queueModel{}

	view := m.queueView()
	plain := ansi.Strip(view)
	if !strings.Contains(plain, "Очередь") {
		t.Fatal("title is missing")
	}
	if !strings.Contains(plain, "Queued track") {
		t.Fatal("manual queue entry is missing")
	}

	for _, line := range strings.Split(plain, "\n") {
		if w := ansi.StringWidth(line); w > 80 {
			t.Fatalf("line is too wide (%d): %q", w, line)
		}
	}
}
