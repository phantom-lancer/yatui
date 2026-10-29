package mainpage

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// The temp directory used to collect one cover file per track forever and a
// log that only ever grew. Both are cut back on start.

func TestCoverCacheIsBounded(t *testing.T) {
	dir := t.TempDir()

	base := time.Now().Add(-time.Hour)
	for i := 0; i < maxCoverCacheFiles+10; i++ {
		path := filepath.Join(dir, "track"+strconv.Itoa(i)+".jpg")
		if err := os.WriteFile(path, []byte("jpg"), 0644); err != nil {
			t.Fatal(err)
		}
		// the later files are the newest
		mod := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	shared := filepath.Join(dir, "cover.jpg")
	if err := os.WriteFile(shared, []byte("shared"), 0644); err != nil {
		t.Fatal(err)
	}
	// the shared cover is the newest one right now
	now := time.Now()
	if err := os.Chtimes(shared, now, now); err != nil {
		t.Fatal(err)
	}

	pruneCoverDir(dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	covers := 0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".jpg" {
			covers++
		}
	}
	if covers != maxCoverCacheFiles {
		t.Fatalf("%d cover files are left, want %d", covers, maxCoverCacheFiles)
	}
	if _, err := os.Stat(shared); err != nil {
		t.Fatal("the shared cover must be kept")
	}
	// the newest stay, the oldest go
	if _, err := os.Stat(filepath.Join(dir, "track0.jpg")); err == nil {
		t.Fatal("the oldest cover must be removed")
	}
	newest := filepath.Join(dir, "track"+strconv.Itoa(maxCoverCacheFiles+9)+".jpg")
	if _, err := os.Stat(newest); err != nil {
		t.Fatal("the newest cover must be kept")
	}
}

func TestTrimLogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yamusic-tui.log")
	big := make([]byte, 5<<20)
	for i := range big {
		big[i] = 'x'
	}
	if err := os.WriteFile(path, big, 0644); err != nil {
		t.Fatal(err)
	}

	trimLogFile(path)

	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size() != 1<<20 {
		t.Fatalf("the log is %d bytes, want 1 MiB", stat.Size())
	}
}

func TestCoverFileForIgnoresMissingCover(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)

	m := &Model{}
	if path := m.coverFileFor(nil); path != "" {
		t.Fatalf("a missing cover must not be reported: %q", path)
	}
}
