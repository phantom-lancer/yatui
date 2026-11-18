package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSaveIsAtomic checks that the config lands on the disk as a whole file:
// the temporary file is renamed, nothing is left behind, and a reload sees the
// newest value.
func TestSaveIsAtomic(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	if err := InitialLoad(); err != nil {
		t.Fatalf("initial load: %s", err)
	}

	Current.Volume = 0.42
	if err := Save(); err != nil {
		t.Fatalf("save: %s", err)
	}

	dir := filepath.Join(home, ".config", DirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %s", err)
	}
	for _, entry := range entries {
		if entry.Name() != "config.yaml" {
			t.Errorf("unexpected file after save: %q", entry.Name())
		}
	}

	if err := Reset(); err != nil {
		t.Fatalf("reload: %s", err)
	}
	if Current.Volume != 0.42 {
		t.Errorf("volume after reload: got %v, want 0.42", Current.Volume)
	}
}

func TestLoadClampsAudioBufferToStableRange(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	dir, err := getDir()
	if err != nil {
		t.Fatalf("config dir: %s", err)
	}

	for _, test := range []struct {
		name  string
		value string
		want  float64
	}{
		{name: "too small", value: "80", want: 180},
		{name: "missing", value: "0", want: defaultConfig.BufferSize},
		{name: "too large", value: "800", want: 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			content := []byte("buffer-size-ms: " + test.value + "\n")
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0600); err != nil {
				t.Fatalf("write config: %s", err)
			}
			loaded, err := load()
			if err != nil {
				t.Fatalf("load config: %s", err)
			}
			if loaded.BufferSize != test.want {
				t.Fatalf("buffer size = %v, want %v", loaded.BufferSize, test.want)
			}
		})
	}
}

// TestSaveLaterCoalesces checks that the delayed write keeps only the newest
// state and that Flush writes it before the app exits.
func TestEqualizerAndWinterSettingsSurviveRestart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	if err := InitialLoad(); err != nil {
		t.Fatalf("initial load: %s", err)
	}

	Current.BufferSize = 240
	Current.WinterSpeed = 65
	Current.WinterDensity = 55
	Current.Equalizer = &Equalizer{
		Enabled: true,
		Preset:  "Rock",
		Gains:   []float64{-3, -2, -1, 0, 1, 2, 3, 2, 1, 0},
	}
	if err := Save(); err != nil {
		t.Fatalf("save settings: %s", err)
	}
	if err := Reset(); err != nil {
		t.Fatalf("reload settings: %s", err)
	}
	if Current.Equalizer == nil || !Current.Equalizer.Enabled || Current.Equalizer.Preset != "Rock" {
		t.Fatalf("equalizer preset did not survive reload: %+v", Current.Equalizer)
	}
	if len(Current.Equalizer.Gains) != 10 || Current.Equalizer.Gains[0] != -3 || Current.Equalizer.Gains[8] != 1 {
		t.Fatalf("equalizer gains did not survive reload: %v", Current.Equalizer.Gains)
	}
	if Current.WinterSpeed != 65 || Current.WinterDensity != 55 || Current.BufferSize != 240 {
		t.Fatalf("settings did not survive reload: speed=%d density=%d buffer=%v", Current.WinterSpeed, Current.WinterDensity, Current.BufferSize)
	}
}

func TestSaveLaterCoalesces(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	if err := InitialLoad(); err != nil {
		t.Fatalf("initial load: %s", err)
	}

	Current.Volume = 0.11
	SaveLater()
	Current.Volume = 0.77
	SaveLater()
	Flush()

	if err := Reset(); err != nil {
		t.Fatalf("reload: %s", err)
	}
	if Current.Volume != 0.77 {
		t.Errorf("volume after flush: got %v, want 0.77", Current.Volume)
	}
}
