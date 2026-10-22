package mainpage

import (
	"testing"

	"github.com/phantom-lancer/yatui/config"
)

// TestWaveSettingsAreApplied checks that the chosen options reach the config:
// the window must not be closed before they are read.
func TestWaveSettingsAreApplied(t *testing.T) {
	previous := config.Current.Wave
	t.Cleanup(func() { config.Current.Wave = previous })

	// the config is written to a temporary directory
	t.Setenv("USERPROFILE", t.TempDir())

	m := &Model{width: 100, height: 30}
	m.stations = &stationsDialogModel{}
	m.wave = &waveSettingsModel{}
	m.wave.setStations(testStationList())
	m.isWaveActive = true

	// the mood is changed
	m.wave.down()
	m.wave.down()
	m.wave.change(1)

	// the apply row
	m.wave.down()
	m.wave.down()
	m.wave.down()

	// the window is closed and the settings are applied
	cmd, handled := m.waveControl(keyMsg("enter"))
	if !handled {
		t.Fatal("the window did not handle the key")
	}
	_ = cmd

	if config.Current.Wave == nil {
		t.Fatal("the settings were not saved")
	}
	if config.Current.Wave.MoodEnergy != "active" {
		t.Errorf("the mood is %q, want active", config.Current.Wave.MoodEnergy)
	}
	if config.Current.Wave.Summary == "" {
		t.Error("the summary of the settings is empty")
	}
}

// TestTracklistDragMovesTheTracks checks that a track can be dragged to
// another place.
func TestTracklistDragMovesTheTracks(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.tracklist = testTracklist(t, []string{"first track", "second track", "third track"})

	items := m.tracklist.Items()
	if len(items) != 3 {
		t.Fatalf("the list has %d rows, want 3", len(items))
	}

	// the first track is taken hold of and moved to the third place
	if !m.tracklist.BeginDrag(0) {
		t.Fatal("the track was not taken")
	}
	m.tracklist.DragTo(2)

	items = m.tracklist.Items()
	if items[2].Track.Title != "first track" {
		t.Fatalf("the track was not moved: %v", items[2].Track.Title)
	}
	if items[0].Track.Title != "second track" || items[1].Track.Title != "third track" {
		t.Fatalf("the other tracks are in the wrong place: %v %v", items[0].Track.Title, items[1].Track.Title)
	}

	if !m.tracklist.EndDrag() {
		t.Error("the drag was not reported as a move")
	}
	if m.tracklist.Dragging() {
		t.Error("the drag is still going")
	}
}

// TestTracklistClickWithoutMovementKeepsTheOrder checks that a click does not
// move anything.
func TestTracklistClickWithoutMovementKeepsTheOrder(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.tracklist = testTracklist(t, []string{"first track", "second track"})

	m.tracklist.BeginDrag(0)
	if m.tracklist.EndDrag() {
		t.Error("a click without a movement was reported as a move")
	}

	items := m.tracklist.Items()
	if items[0].Track.Title != "first track" {
		t.Fatalf("the track was moved by a click: %v", items[0].Track.Title)
	}
}
