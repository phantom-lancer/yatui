package tracker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/phantom-lancer/yatui/cache"
	"github.com/phantom-lancer/yatui/config"
)

// prepareLoudnessCache points the cache to an empty directory and clears the
// in-memory state.
func prepareLoudnessCache(t *testing.T) {
	t.Helper()

	previous := config.Current.CacheDir
	config.Current.CacheDir = t.TempDir()

	resetLoudnessCache()
	t.Cleanup(func() {
		config.Current.CacheDir = previous
		resetLoudnessCache()
	})
}

func resetLoudnessCache() {
	loudnessMux.Lock()
	defer loudnessMux.Unlock()
	loudnessCache = nil
	loudnessLoaded = false
}

func TestLoudnessCacheRoundTrip(t *testing.T) {
	prepareLoudnessCache(t)

	if _, ok := lookupLoudness("missing"); ok {
		t.Fatal("an empty cache returned an entry")
	}

	storeLoudness("123", loudnessEntry{Loudness: -9.5, Peak: 0.98})
	if entry, ok := lookupLoudness("123"); !ok || entry.Loudness != -9.5 || entry.Peak != 0.98 {
		t.Fatalf("stored entry was lost: %+v, %v", entry, ok)
	}

	// the file is written in the background: wait for it instead of sleeping
	waitLoudnessCache()
	saveLoudnessCache()

	// a fresh start of the program has to see the stored value
	resetLoudnessCache()
	entry, ok := lookupLoudness("123")
	if !ok {
		t.Fatal("the cache was not restored from the disk")
	}
	if entry.Loudness != -9.5 || entry.Peak != 0.98 {
		t.Fatalf("restored entry differs: %+v", entry)
	}
}

func TestLoudnessCacheIgnoresBrokenFile(t *testing.T) {
	prepareLoudnessCache(t)

	// a broken file must not break the playback
	dir, err := cache.Dir()
	if err != nil {
		t.Skip("no cache directory")
	}
	if err := os.WriteFile(filepath.Join(dir, _LOUDNESS_CACHE_FILE), []byte("{not a json"), 0644); err != nil {
		t.Fatal(err)
	}

	resetLoudnessCache()
	if _, ok := lookupLoudness("123"); ok {
		t.Fatal("a broken cache returned an entry")
	}
}
