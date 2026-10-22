package mainpage

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/api"
)

// TestOnlyOneTrackIsMarkedAsPlaying makes sure that the marker of the
// playing track is not left on the track that played before.
func TestOnlyOneTrackIsMarkedAsPlaying(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.playlists = testPlaylists(t, []string{"my wave"})
	m.tracklist = testTracklist(t, []string{"first track", "second track"})

	playlistItem := m.playlists.SelectedItem()
	playlistItem.Tracks = []api.Track{testTrack("p1", "First"), testTrack("p2", "Second")}
	m.playlists.SetItem(0, playlistItem)
	m.displayPlaylist(playlistItem)

	m.currentPlaylistIndex = 0
	playlistItem.CurrentTrack = 0
	m.playlists.SetItem(0, playlistItem)

	m.indicateCurrentTrackPlaying(true)

	marked := 0
	for _, item := range m.tracklist.Items() {
		if item.IsPlaying {
			marked++
		}
	}
	if marked != 1 {
		t.Fatalf("%d tracks are marked as playing, want 1", marked)
	}

	// the next track plays: the marker moves and the old one is gone
	playlistItem.CurrentTrack = 1
	m.playlists.SetItem(0, playlistItem)
	m.indicateCurrentTrackPlaying(true)

	marked = 0
	for _, item := range m.tracklist.Items() {
		if item.IsPlaying {
			marked++
			if item.Track.Id != "p2" {
				t.Errorf("the marker is on %q, want p2", item.Track.Id)
			}
		}
	}
	if marked != 1 {
		t.Fatalf("%d tracks are marked as playing, want 1", marked)
	}
}

// TestMotionWithTheButtonHeldMovesTheTrack makes sure that the drag of a
// track works: the terminals report the motion with the button pressed.
func TestMotionWithTheButtonHeldMovesTheTrack(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.tracklist = testTracklist(t, []string{"first track", "second track", "third track"})

	// the press takes hold of the first track
	m.tracklist.BeginDrag(0)

	// the motion with the button held moves it
	m.tracklist.DragTo(2)
	items := m.tracklist.Items()
	if items[2].Track.Title != "first track" {
		t.Fatalf("the track was not moved: %v", items[2].Track.Title)
	}
}

// TestReleaseWithoutAButtonEndsTheDrag makes sure that the drag ends even if
// the terminal reports the release without a button.
func TestReleaseWithoutAButtonEndsTheDrag(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.tracklist = testTracklist(t, []string{"first track", "second track"})
	m.tracklist.BeginDrag(0)

	if !m.tracklist.Dragging() {
		t.Fatal("the drag did not start")
	}
	if m.tracklist.EndDrag() {
		t.Error("a drag without a movement was reported as a move")
	}
	if m.tracklist.Dragging() {
		t.Error("the drag is still going")
	}
}

var _ = tea.KeyMsg{}
