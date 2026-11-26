package cache

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCommitPublishesFile checks the atomic cache write: a track is invisible
// until Commit, and a discarded file leaves nothing behind.
func TestCommitPublishesFile(t *testing.T) {
	dir := t.TempDir()
	SetDir(dir)
	t.Cleanup(func() { SetDir("") })

	file, err := Write("12345")
	if err != nil {
		t.Fatalf("write: %s", err)
	}
	if _, err := file.Write([]byte("track data")); err != nil {
		t.Fatalf("write data: %s", err)
	}

	if Exists("12345") {
		t.Fatal("a partial file is visible as a cached track")
	}

	if err := Commit(file, "12345"); err != nil {
		t.Fatalf("commit: %s", err)
	}
	if !Exists("12345") {
		t.Fatal("a committed track is missing")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %s", err)
	}
	if len(entries) != 1 || entries[0].Name() != "12345.mp3" {
		t.Fatalf("cache dir: %v", entries)
	}

	file, err = Write("999")
	if err != nil {
		t.Fatalf("write: %s", err)
	}
	file.Write([]byte("x"))
	Discard(file)

	if Exists("999") {
		t.Fatal("a discarded file is visible as a cached track")
	}
	entries, _ = os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("leftovers after discard: %v", entries)
	}

	// the path is not a flat name: the file lives in the cache directory
	if _, err := os.Stat(filepath.Join(dir, "12345.mp3")); err != nil {
		t.Fatalf("committed file: %s", err)
	}
}
