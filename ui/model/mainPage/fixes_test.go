package mainpage

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
)

// TestToastsExpireWithTheMenuOpen makes sure that the notifications are
// pruned even when a menu swallows the rest of the messages.
func TestToastsExpireWithTheMenuOpen(t *testing.T) {
	m := &Model{width: 100, height: 30}
	m.contextMenu = &contextMenu{items: []contextMenuItem{{label: "one"}}}
	m.toasts = append(m.toasts, toast{kind: "a", text: "a", expires: time.Now().Add(-time.Second)})

	m.Update(toastExpiredMsg{})

	if len(m.toasts) != 0 {
		t.Fatalf("the notification was not pruned: %v", m.toasts)
	}
	if m.contextMenu == nil {
		t.Error("the menu was closed by the timer")
	}
}

// TestQueueTracksCanBeDragged checks that a track of the queue can be moved
// with the mouse.
func TestQueueTracksCanBeDragged(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.manualQueue = []api.Track{
		testTrack("m1", "First"),
		testTrack("m2", "Second"),
		testTrack("m3", "Third"),
	}
	m.queue = &queueModel{}

	// the second track is taken hold of and dropped after the third one
	m.queue.dragFrom = 1
	m.queue.index = 2
	m.finishQueueDrag()

	if m.manualQueue[0].Id != "m1" || m.manualQueue[1].Id != "m3" || m.manualQueue[2].Id != "m2" {
		t.Fatalf("the track was not moved: %v %v %v",
			m.manualQueue[0].Id, m.manualQueue[1].Id, m.manualQueue[2].Id)
	}
	if len(m.manualQueue) != 3 {
		t.Fatalf("the queue lost a track: %d", len(m.manualQueue))
	}
	if m.queue.dragFrom != -1 {
		t.Error("the drag was not finished")
	}
}

// TestNextTracksAreShownInTheTrackList makes sure that the tracks of the
// queue are visible and that the indexes of the list and of the playlist are
// still in sync.
func TestNextTracksAreShownInTheTrackList(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.manualQueue = []api.Track{testTrack("m1", "Next one")}
	m.playlists = testPlaylists(t, []string{"my wave"})
	m.tracklist = testTracklist(t, []string{"x"})

	playlistItem := m.playlists.SelectedItem()
	playlistItem.Tracks = []api.Track{testTrack("p1", "First"), testTrack("p2", "Second")}
	playlistItem.SelectedTrack = 1
	m.playlists.SetItem(0, playlistItem)

	m.displayPlaylist(playlistItem)

	items := m.tracklist.Items()
	if len(items) != 3 {
		t.Fatalf("the list has %d rows, want 3", len(items))
	}
	if !items[0].IsNext || items[0].Track.Id != "m1" {
		t.Errorf("the track of the queue is not at the top: %+v", items[0])
	}
	if items[1].Track.Id != "p1" || items[2].Track.Id != "p2" {
		t.Errorf("the tracks of the playlist are in the wrong place")
	}

	// the index of the list is shifted by the tracks of the queue
	if got := m.playlistTrackIndex(2); got != 1 {
		t.Errorf("the index of the playlist is %d, want 1", got)
	}
	if got := m.tracklistIndex(1); got != 2 {
		t.Errorf("the index of the list is %d, want 2", got)
	}
	if got := m.tracklist.SelectedItem().Track.Id; got != "p2" {
		t.Errorf("the selected track is %q, want p2", got)
	}
}

// TestWaveSettingsSummary checks the description of the settings of the wave.
func TestWaveSettingsSummary(t *testing.T) {
	previous := config.Current.Wave
	t.Cleanup(func() { config.Current.Wave = previous })

	config.Current.Wave = &config.Wave{
		Diversity:  "discover",
		MoodEnergy: "active",
		Language:   "russian",
	}

	summary := waveSettingsSummary()
	if summary == "" {
		t.Fatal("the summary is empty")
	}
	for _, want := range []string{"discover", "active", "russian"} {
		if !contains(summary, want) {
			t.Errorf("the summary %q does not mention %q", summary, want)
		}
	}
}

// contains reports whether the text has the part.
func contains(text, part string) bool {
	return len(text) >= len(part) && (text == part || len(part) == 0 ||
		indexOf(text, part) >= 0)
}

func indexOf(text, part string) int {
	for i := 0; i+len(part) <= len(text); i++ {
		if text[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}

var _ = tea.KeyMsg{}
