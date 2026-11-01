package mainpage

import (
	"fmt"
	"testing"
	"time"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
)

// TestManualPlayingShiftsTheTrackList makes sure the track of the queue that
// is playing right now is shown at the top of the list, so the row under the
// cursor matches the playlist again.
func TestManualPlayingShiftsTheTrackList(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.playlists = testPlaylists(t, []string{"my wave"})
	m.tracklist = testTracklist(t, []string{"x"})

	pl := m.playlists.SelectedItem()
	pl.Tracks = []api.Track{testTrack("p1", "First"), testTrack("p2", "Second")}
	m.playlists.SetItem(0, pl)

	manual := testTrack("m1", "Manual")
	m.manualPlaying = &manual
	m.displayPlaylist(pl)

	items := m.tracklist.Items()
	if len(items) != 3 {
		t.Fatalf("the list has %d rows, want 3", len(items))
	}
	if items[0].Track.Id != "m1" {
		t.Errorf("the manual track is not at the top: %+v", items[0])
	}
	if items[1].Track.Id != "p1" || items[2].Track.Id != "p2" {
		t.Errorf("the playlist tracks are in the wrong place")
	}
	if got := m.nextCount(); got != 1 {
		t.Errorf("nextCount = %d, want 1", got)
	}
	if got := m.playlistTrackIndex(1); got != 0 {
		t.Errorf("the first row of the playlist is mapped to %d, want 0", got)
	}
}

// TestShuffleWritesTheClickedPlaylist checks that shuffling a playlist that is
// not open does not touch the list on the screen and does not clone it either.
func TestShuffleWritesTheClickedPlaylist(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.playlists = testPlaylists(t, []string{"one", "two"})
	m.playlists.SetSize(32, 24)

	items := m.playlists.Items()
	for i := 0; i < 20; i++ {
		items[0].Tracks = append(items[0].Tracks, testTrack(fmt.Sprintf("a%d", i), "A"))
		items[1].Tracks = append(items[1].Tracks, testTrack(fmt.Sprintf("b%d", i), "B"))
	}
	m.playlists.SetItem(0, items[0])
	m.playlists.SetItem(1, items[1])
	m.playlists.Select(0)

	// the open playlist is the first one, the clicked one is the second.
	// The returned command is nil, the sidebar is changed in place.
	m.shufflePlaylist(m.playlists.Items()[1])

	after := m.playlists.Items()
	if len(m.playlists.Items()) != 2 {
		t.Fatalf("the sidebar has %d items, want 2", len(after))
	}
	if len(after[0].Tracks) != 20 || after[0].Tracks[0].Id != "a0" || after[0].Tracks[19].Id != "a19" {
		t.Error("the open playlist was changed by the shuffle of another one")
	}

	if len(after[1].Tracks) != 20 {
		t.Fatalf("the shuffled playlist has %d tracks, want 20", len(after[1].Tracks))
	}
	seen := make(map[string]bool, 20)
	for _, track := range after[1].Tracks {
		seen[track.Id] = true
	}
	for i := 0; i < 20; i++ {
		if !seen[fmt.Sprintf("b%d", i)] {
			t.Fatalf("the track b%d was lost by the shuffle", i)
		}
	}
}

// TestDiscordAfkHidesTheCard checks the AFK timeout: it counts the absence of
// the user input, not the playback state.
func TestDiscordAfkHidesTheCard(t *testing.T) {
	previous := config.Current.Discord
	t.Cleanup(func() { config.Current.Discord = previous })
	config.Current.Discord = &config.Discord{Enabled: true, AfkTimeout: 5}

	m := &Model{lastInput: time.Now().Add(-6 * time.Minute)}
	if !m.discordAfk() {
		t.Error("the user is away, the card must be hidden")
	}

	m.lastInput = time.Now().Add(-time.Minute)
	if m.discordAfk() {
		t.Error("the user is here, the card must be shown")
	}

	// the timer is off with a zero timeout and before the first input
	m.lastInput = time.Time{}
	if m.discordAfk() {
		t.Error("the timeout must not start before the first input")
	}
	config.Current.Discord.AfkTimeout = 0
	m.lastInput = time.Now().Add(-time.Hour)
	if m.discordAfk() {
		t.Error("a zero timeout disables the hiding")
	}
}

// TestTrackSearchAcceptsSpaces checks that a space is typed into the quick
// search query instead of stopping it: tea.KeySpace.String() is " ".
func TestTrackSearchAcceptsSpaces(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.tracklist = testTracklist(t, []string{"Space song", "Other song"})

	m.openTrackSearch()
	for _, r := range []string{"s", "p", "a", "c", "e", " ", "s"} {
		if _, handled := m.trackSearchControl(keyMsg(r)); !handled {
			t.Fatalf("key %q was not handled", r)
		}
	}

	if m.trackSearch.query != "space s" {
		t.Fatalf("query is %q, want %q", m.trackSearch.query, "space s")
	}
	if len(m.trackSearch.results) != 1 {
		t.Fatalf("the query found %d tracks, want 1", len(m.trackSearch.results))
	}
}

// TestQueuePlaylistPutsItAfterTheCurrentTrack checks the context menu action
// that queues a whole playlist.
func TestQueuePlaylistPutsItAfterTheCurrentTrack(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.playlists = testPlaylists(t, []string{"list"})
	m.tracklist = testTracklist(t, []string{"x"})

	pl := m.playlists.SelectedItem()
	pl.Tracks = []api.Track{testTrack("q1", "One"), testTrack("q2", "Two")}
	m.playlists.SetItem(0, pl)

	if cmd := m.queuePlaylist(pl); cmd != nil {
		t.Error("queueing a playlist needs no command")
	}
	if len(m.manualQueue) != 2 || m.manualQueue[0].Id != "q1" || m.manualQueue[1].Id != "q2" {
		t.Fatalf("unexpected queue: %v", m.manualQueue)
	}
}
