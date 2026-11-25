package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/phantom-lancer/yatui/api"
)

func offlineTrack(id, title string) api.Track {
	track := api.Track{Id: id, Title: title, Available: true, DurationMs: 1000}
	return track
}

// writeCachedTrack stores a fake cached file, the way a finished download
// would leave it.
func writeCachedTrack(t *testing.T, id string, size int) {
	t.Helper()
	file, err := Write(id)
	if err != nil {
		t.Fatalf("write %s: %s", id, err)
	}
	if size > 0 {
		if _, err := file.Write(make([]byte, size)); err != nil {
			t.Fatalf("write data: %s", err)
		}
	}
	if err := Commit(file, id); err != nil {
		t.Fatalf("commit %s: %s", id, err)
	}
}

func TestOfflineCatalogKeepsOnlyCachedTracks(t *testing.T) {
	SetDir(t.TempDir())
	t.Cleanup(func() { SetDir("") })

	writeCachedTrack(t, "cached", 128)

	playlists := []OfflinePlaylist{
		{Name: "likes", Kind: 2, Tracks: []api.Track{
			offlineTrack("cached", "Cached"),
			offlineTrack("missing", "Missing"),
		}},
		{Name: "empty", Kind: 2, Tracks: []api.Track{offlineTrack("missing", "Missing")}},
	}
	if err := SaveOfflineCatalog(playlists); err != nil {
		t.Fatalf("save: %s", err)
	}

	loaded, err := LoadOfflineCatalog()
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("the catalog has %d playlists, want 1: %+v", len(loaded), loaded)
	}
	if len(loaded[0].Tracks) != 1 || loaded[0].Tracks[0].Id != "cached" {
		t.Fatalf("unexpected tracks: %+v", loaded[0].Tracks)
	}
	if !loaded[0].Tracks[0].Available {
		t.Fatal("a cached track is not marked available")
	}

	// a file removed behind the catalog must disappear from the list
	if err := Remove("cached"); err != nil {
		t.Fatalf("remove: %s", err)
	}
	loaded, err = LoadOfflineCatalog()
	if err != nil {
		t.Fatalf("load after remove: %s", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("a removed track is still listed: %+v", loaded)
	}
}

func TestOfflineCatalogMissingFileIsNotAnError(t *testing.T) {
	SetDir(t.TempDir())
	t.Cleanup(func() { SetDir("") })

	loaded, err := LoadOfflineCatalog()
	if err != nil {
		t.Fatalf("a missing catalog is an error: %s", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("unexpected playlists: %+v", loaded)
	}
}

func TestPrunePartialsRemovesInterruptedDownloads(t *testing.T) {
	dir := t.TempDir()
	SetDir(dir)
	t.Cleanup(func() { SetDir("") })

	// a failed download leaves a temporary file behind
	part := filepath.Join(dir, "track.123.part")
	if err := os.WriteFile(part, []byte("half"), 0600); err != nil {
		t.Fatal(err)
	}
	writeCachedTrack(t, "good", 64)

	if removed := PrunePartials(); removed != 1 {
		t.Fatalf("removed %d files, want 1", removed)
	}
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Fatal("the partial file is still there")
	}
	if !IntegrityCheck("good") {
		t.Fatal("a committed track was removed")
	}
	if IntegrityCheck("missing") {
		t.Fatal("a missing track passed the integrity check")
	}
}

func TestEmptyFileIsNotATrack(t *testing.T) {
	dir := t.TempDir()
	SetDir(dir)
	t.Cleanup(func() { SetDir("") })

	if err := os.WriteFile(filepath.Join(dir, "empty.mp3"), nil, 0600); err != nil {
		t.Fatal(err)
	}

	tracks, err := ListTracks()
	if err != nil {
		t.Fatalf("list: %s", err)
	}
	if len(tracks) != 0 {
		t.Fatalf("an empty file is listed as a track: %+v", tracks)
	}
	if IntegrityCheck("empty") {
		t.Fatal("an empty file passed the integrity check")
	}
}
