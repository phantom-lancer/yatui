package tracker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/phantom-lancer/yatui/cache"
	"github.com/phantom-lancer/yatui/log"
)

// _LOUDNESS_CACHE_FILE keeps the measured loudness of the tracks between the
// runs: the head of a track is only an approximation of the whole track, so
// the correct value is measured while the track plays and used from the
// second play on. The file lives next to the cached tracks.
const _LOUDNESS_CACHE_FILE = "loudness.json"

// loudnessEntry is the measured loudness of a whole track.
type loudnessEntry struct {
	Loudness float64 `json:"loudness"`
	Peak     float64 `json:"peak"`
}

var (
	loudnessMux    sync.Mutex
	loudnessCache  map[string]loudnessEntry
	loudnessLoaded bool
	loudnessSave   sync.Mutex
	loudnessSaved  sync.WaitGroup
)

// lookupLoudness returns the measured loudness of a track, if it was measured
// before.
func lookupLoudness(trackID string) (loudnessEntry, bool) {
	if len(trackID) == 0 {
		return loudnessEntry{}, false
	}

	loudnessMux.Lock()
	defer loudnessMux.Unlock()

	loadLoudnessCache()
	entry, ok := loudnessCache[trackID]
	return entry, ok
}

// storeLoudness remembers the measured loudness of a track and writes the
// cache to the disk in the background: the call itself may happen on the audio
// thread.
func storeLoudness(trackID string, entry loudnessEntry) {
	if len(trackID) == 0 {
		return
	}

	loudnessMux.Lock()
	loadLoudnessCache()
	previous, exists := loudnessCache[trackID]
	loudnessCache[trackID] = entry
	loudnessMux.Unlock()

	if exists && previous == entry {
		return
	}

	loudnessSaved.Add(1)
	go func() {
		defer loudnessSaved.Done()
		saveLoudnessCache()
	}()
}

// waitLoudnessCache waits for the pending writes of the cache to finish.
func waitLoudnessCache() {
	loudnessSaved.Wait()
}

// loadLoudnessCache reads the file once. loudnessMux has to be held.
func loadLoudnessCache() {
	if loudnessLoaded {
		return
	}
	loudnessLoaded = true
	loudnessCache = make(map[string]loudnessEntry)

	dir, err := cache.Dir()
	if err != nil {
		return
	}

	data, err := os.ReadFile(filepath.Join(dir, _LOUDNESS_CACHE_FILE))
	if err != nil {
		return
	}

	if err := json.Unmarshal(data, &loudnessCache); err != nil {
		log.Print(log.LVL_WARNIGN, "normalization: unable to read the loudness cache: %s", err)
		loudnessCache = make(map[string]loudnessEntry)
	}
}

// saveLoudnessCache writes the cache to the disk. The file is written next to
// its destination and renamed, so a half written cache never survives a crash.
func saveLoudnessCache() {
	// the writes are serialized: two of them at the same time could leave an
	// older version of the file behind
	loudnessSave.Lock()
	defer loudnessSave.Unlock()

	dir, err := cache.Dir()
	if err != nil {
		return
	}

	loudnessMux.Lock()
	data, err := json.Marshal(loudnessCache)
	loudnessMux.Unlock()
	if err != nil {
		return
	}

	path := filepath.Join(dir, _LOUDNESS_CACHE_FILE)
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0644); err != nil {
		log.Print(log.LVL_WARNIGN, "normalization: unable to update the loudness cache: %s", err)
		return
	}

	if err := os.Rename(temporary, path); err != nil {
		log.Print(log.LVL_WARNIGN, "normalization: unable to update the loudness cache: %s", err)
		os.Remove(temporary)
	}
}
