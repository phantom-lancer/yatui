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

// TestSaveLaterCoalesces checks that the delayed write keeps only the newest
// state and that Flush writes it before the app exits.
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
