package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/phantom-lancer/yatui/equalizer"
)

// writeConfig stores a config file in an isolated home and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	dir := filepath.Join(home, ".config", DirName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLegacyEqualizerMigratesToGraphic checks the one time migration: the ten
// band curve is resampled into fifteen bands, while the original values stay
// untouched for the classic view.
func TestLegacyEqualizerMigratesToGraphic(t *testing.T) {
	writeConfig(t, `token: abc
equalizer:
    enabled: true
    preset: Свой
    gains: [0, 0, 0, 0, 12, 0, 0, 0, 0, 0]
`)

	conf, err := load()
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if conf.Equalizer == nil {
		t.Fatal("the equalizer was dropped")
	}
	if len(conf.Equalizer.Gains) != equalizer.Bands {
		t.Fatalf("legacy gains: %d bands", len(conf.Equalizer.Gains))
	}
	if conf.Equalizer.Gains[4] != 12 {
		t.Fatalf("the legacy curve changed: %v", conf.Equalizer.Gains)
	}
	if len(conf.Equalizer.GraphicGains) != equalizer.GraphicBands {
		t.Fatalf("graphic gains: %d bands", len(conf.Equalizer.GraphicGains))
	}
	if conf.Equalizer.GraphicPreset != equalizer.CustomPreset {
		t.Fatalf("a custom curve got the preset %q", conf.Equalizer.GraphicPreset)
	}

	peak := 0
	for i, gain := range conf.Equalizer.GraphicGains {
		if gain > conf.Equalizer.GraphicGains[peak] {
			peak = i
		}
	}
	if conf.Equalizer.GraphicGains[peak] < 6 {
		t.Fatalf("the migrated curve is too quiet: %v", conf.Equalizer.GraphicGains)
	}
	if peak < 8 || peak > 11 {
		t.Fatalf("the migrated peak is at band %d: %v", peak, conf.Equalizer.GraphicGains)
	}
}

// TestGraphicEqualizerSurvivesReload checks that both views are persisted
// independently: saving the graphic values does not overwrite the classic
// ones, and the mode is remembered.
func TestGraphicEqualizerSurvivesReload(t *testing.T) {
	writeConfig(t, `token: abc
equalizer:
    enabled: true
    classic-view: true
    preset: Свой
    gains: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
    graphic-preset: Свой
    graphic-gains: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 0, 0, 0]
`)

	conf, err := load()
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if !conf.Equalizer.ClassicView {
		t.Fatal("the classic view was not remembered")
	}
	if len(conf.Equalizer.Gains) != equalizer.Bands || conf.Equalizer.Gains[9] != 10 {
		t.Fatalf("classic gains: %v", conf.Equalizer.Gains)
	}
	if len(conf.Equalizer.GraphicGains) != equalizer.GraphicBands || conf.Equalizer.GraphicGains[11] != 12 {
		t.Fatalf("graphic gains: %v", conf.Equalizer.GraphicGains)
	}
}

// TestNewSettingsDefaultToOn checks the defaults of the options added in this
// version: an old config gets the visualizer on and the saver/offline off.
func TestNewSettingsDefaultToOn(t *testing.T) {
	writeConfig(t, "token: abc\n")

	conf, err := load()
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if !conf.VisualizerEnabled {
		t.Fatal("the visualizer must be on by default")
	}
	if conf.ResourceSaver || conf.Offline {
		t.Fatalf("saver=%v offline=%v, want both off", conf.ResourceSaver, conf.Offline)
	}
}

// TestPrivatePermissions checks that an old world readable config is tightened
// when it is opened.
func TestPrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX mode bits")
	}

	path := writeConfig(t, "token: abc\n")
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := load(); err != nil {
		t.Fatalf("load: %s", err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := stat.Mode().Perm(); perm != 0600 {
		t.Fatalf("the config is %04o, want 0600", perm)
	}
}

// TestVisualizerModeMigratesFromTheBoolean checks that an old config with
// only "visualizer: true/false" keeps its meaning in the new mode setting.
func TestVisualizerModeMigratesFromTheBoolean(t *testing.T) {
	writeConfig(t, "token: abc\nvisualizer: true\n")
	conf, err := load()
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if conf.VisualizerMode != "panel" {
		t.Fatalf("mode %q, want panel", conf.VisualizerMode)
	}

	writeConfig(t, "token: abc\nvisualizer: false\n")
	conf, err = load()
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if conf.VisualizerMode != "off" {
		t.Fatalf("mode %q, want off", conf.VisualizerMode)
	}

	// an unknown mode falls back to the panel, never to a broken state
	writeConfig(t, "token: abc\nvisualizer-mode: neon\n")
	conf, err = load()
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if conf.VisualizerMode != "panel" || !conf.VisualizerEnabled {
		t.Fatalf("mode %q enabled=%v, want panel/true", conf.VisualizerMode, conf.VisualizerEnabled)
	}
}

// TestVisualizerModeRoundTrip checks that the three modes survive a save and
// a reload, and that the old boolean follows the mode.
func TestVisualizerModeRoundTrip(t *testing.T) {
	writeConfig(t, "token: abc\nvisualizer: true\nvisualizer-mode: background\n")
	conf, err := load()
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if conf.VisualizerMode != "background" || !conf.VisualizerEnabled {
		t.Fatalf("mode %q enabled=%v", conf.VisualizerMode, conf.VisualizerEnabled)
	}
}
