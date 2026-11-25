package mainpage

import (
	"testing"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/cache"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/equalizer"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
	"github.com/phantom-lancer/yatui/ui/components/tracker"
	"github.com/phantom-lancer/yatui/visualizer"
)

// withConfig restores the mutable config after a test.
func withConfig(t *testing.T, change func()) {
	t.Helper()
	previous := config.Current
	t.Cleanup(func() { config.Current = previous })
	change()
}

func newTestPlaylists() *playlist.Model {
	return playlist.New(nil, "yatui")
}

func TestVisualizerHiddenInResourceSaverMode(t *testing.T) {
	withConfig(t, func() {
		config.Current.VisualizerEnabled = true
		config.Current.ResourceSaver = false
	})

	m := &Model{width: 120, height: 40}
	m.playlists = newTestPlaylists()
	if !m.visualizerVisible() {
		t.Fatal("the visualizer must be visible on a normal screen")
	}

	config.Current.ResourceSaver = true
	if m.visualizerVisible() {
		t.Fatal("the resource saver must hide the visualizer")
	}
	if m.visualizerEnabled() {
		t.Fatal("the resource saver must disable the visualizer")
	}
}

func TestVisualizerHiddenOnSmallScreens(t *testing.T) {
	withConfig(t, func() {
		config.Current.VisualizerEnabled = true
		config.Current.ResourceSaver = false
	})

	m := &Model{width: 40, height: 30}
	m.playlists = newTestPlaylists()
	if m.visualizerVisible() {
		t.Fatal("the visualizer must not take the room of a narrow screen")
	}

	m.width, m.height = 120, 10
	if m.visualizerVisible() {
		t.Fatal("the visualizer must not take the room of a short screen")
	}
}

func TestToggleVisualizerReleasesTheWorker(t *testing.T) {
	withConfig(t, func() {
		config.Current.VisualizerEnabled = true
		config.Current.ResourceSaver = false
	})

	m := &Model{width: 120, height: 40}
	m.visualizer = visualizer.New()
	m.visualizer.SetEnabled(true)
	if !m.visualizer.Enabled() {
		t.Fatal("the worker did not start")
	}

	m.toggleVisualizer()
	if config.Current.VisualizerEnabled {
		t.Fatal("the setting did not change")
	}
	if m.visualizer.Enabled() {
		t.Fatal("the worker is still running after disabling")
	}
}

func TestEqualizerViewsKeepTheirOwnGains(t *testing.T) {
	withConfig(t, func() {
		config.Current.Equalizer = &config.Equalizer{
			Enabled:       true,
			Preset:        equalizer.CustomPreset,
			Gains:         []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			ClassicView:   false,
			GraphicPreset: equalizer.CustomPreset,
			GraphicGains:  []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 0, 0, 0},
		}
	})

	m := &Model{width: 120, height: 40}
	m.eq = &eqModel{
		enabled:       true,
		preset:        equalizer.CustomPreset,
		gains:         equalizer.NormalizeGains(config.Current.Equalizer.Gains),
		graphic:       true,
		graphicPreset: equalizer.CustomPreset,
		graphicGains:  equalizer.NormalizeGraphicGains(config.Current.Equalizer.GraphicGains),
		index:         -1,
	}

	m.toggleEQView()

	if m.eq.graphic {
		t.Fatal("the view did not switch to classic")
	}
	if m.eq.gains[9] != 10 {
		t.Fatalf("the classic gains changed: %v", m.eq.gains)
	}
	if m.eq.graphicGains[11] != 12 {
		t.Fatalf("the graphic gains changed: %v", m.eq.graphicGains)
	}
	if config.Current.Equalizer == nil || !config.Current.Equalizer.ClassicView {
		t.Fatal("the classic view was not saved")
	}
	if config.Current.Equalizer.Gains[9] != 10 || config.Current.Equalizer.GraphicGains[11] != 12 {
		t.Fatal("saving one view overwrote the other")
	}

	// and back: the graphic values are still there
	m.toggleEQView()
	if !m.eq.graphic {
		t.Fatal("the view did not switch back")
	}
	if m.eq.graphicGains[11] != 12 || config.Current.Equalizer.GraphicGains[11] != 12 {
		t.Fatal("the graphic gains were lost after the round trip")
	}
}

func TestOpenEqualizerMigratesLegacyGains(t *testing.T) {
	withConfig(t, func() {
		config.Current.Equalizer = &config.Equalizer{
			Enabled: true,
			Preset:  equalizer.CustomPreset,
			Gains:   []float64{0, 0, 0, 0, 12, 0, 0, 0, 0, 0},
		}
	})

	m := &Model{width: 120, height: 40}
	m.openEqualizer()

	if m.eq == nil || !m.eq.graphic {
		t.Fatal("a config without the graphic values must open the graphic view")
	}
	peak := 0
	for i, gain := range m.eq.graphicGains {
		if gain > m.eq.graphicGains[peak] {
			peak = i
		}
	}
	if m.eq.graphicGains[peak] < 6 {
		t.Fatalf("the legacy curve was not migrated: %v", m.eq.graphicGains)
	}
}

func TestSettingsHaveTheNewRows(t *testing.T) {
	withConfig(t, func() {
		config.Current.VisualizerEnabled = true
		config.Current.ResourceSaver = false
		config.Current.Offline = false
		config.Current.Equalizer = &config.Equalizer{ClassicView: false}
	})

	m := &Model{width: 120, height: 40}
	m.tracker = &tracker.Model{}

	items := m.buildSettings()
	wanted := map[string]bool{
		"Визуализатор «Моей волны»":    false,
		"Экономия ресурсов":            false,
		"Классический вид эквалайзера": false,
		"Офлайн-режим":                 false,
	}
	for _, item := range items {
		if _, ok := wanted[item.label]; ok {
			wanted[item.label] = true
		}
	}
	for label, found := range wanted {
		if !found {
			t.Errorf("the settings row %q is missing", label)
		}
	}
}

func TestResourceSaverStopsTheVisualizer(t *testing.T) {
	withConfig(t, func() {
		config.Current.ResourceSaver = false
		config.Current.VisualizerEnabled = true
	})

	m := &Model{width: 120, height: 40}
	m.visualizer = visualizer.New()
	m.visualizer.SetEnabled(true)
	m.winterRunning = true

	config.Current.ResourceSaver = true
	m.applyResourceSaving()

	if m.visualizer.Enabled() {
		t.Fatal("the visualizer worker survived the resource saver")
	}
	if m.winterRunning {
		t.Fatal("the snow timer survived the resource saver")
	}
}

func TestOfflineCatalogIsListedWithoutNetwork(t *testing.T) {
	withConfig(t, func() {
		config.Current.Offline = true
	})

	dir := t.TempDir()
	cache.SetDir(dir)
	t.Cleanup(func() { cache.SetDir("") })

	file, err := cache.Write("track-1")
	if err != nil {
		t.Fatalf("cache write: %s", err)
	}
	if _, err := file.Write([]byte("not really mp3")); err != nil {
		t.Fatalf("cache data: %s", err)
	}
	if err := cache.Commit(file, "track-1"); err != nil {
		t.Fatalf("cache commit: %s", err)
	}

	saved := []cache.OfflinePlaylist{{
		Name: "Любимые",
		Kind: 2,
		Tracks: []api.Track{{
			Id: "track-1", Title: "Cached", Available: true, DurationMs: 1000,
		}},
	}}
	if err := cache.SaveOfflineCatalog(saved); err != nil {
		t.Fatalf("save catalog: %s", err)
	}

	m := &Model{width: 120, height: 40}
	m.playlists = newTestPlaylists()
	m.cachedTracksMap = map[string]bool{}
	m.loadOfflineCatalog()

	items := m.playlists.Items()
	if len(items) < 2 {
		t.Fatalf("the offline sidebar has %d items, want local + saved", len(items))
	}
	if items[0].Name != "local" {
		t.Fatalf("the first item is %q, want local", items[0].Name)
	}
	if !m.cachedTracksMap["track-1"] {
		t.Fatal("the cached track is not marked as cached")
	}

	found := false
	for _, item := range items {
		if item.Name == "Любимые" && len(item.Tracks) == 1 && item.Tracks[0].Id == "track-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the saved playlist is missing: %+v", items)
	}
}
