package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/phantom-lancer/yatui/api"
)

// The offline catalog remembers which playlists the user had open and which
// of their tracks are already in the cache. It is not a second copy of the
// music: every entry is checked against the cache directory before it is
// listed, so a removed or truncated file never looks playable.

const offlineCatalogFile = "offline.json"

// OfflinePlaylist is one playlist of the offline catalog.
type OfflinePlaylist struct {
	Name   string      `json:"name"`
	Kind   uint64      `json:"kind"`
	Tracks []api.Track `json:"tracks"`
}

var offlineMu sync.Mutex

// SaveOfflineCatalog stores the playlists, keeping only the tracks that are
// actually in the cache right now. The write is atomic: an interrupted save
// leaves the previous catalog intact.
func SaveOfflineCatalog(playlists []OfflinePlaylist) error {
	dir, err := getCacheDir()
	if err != nil {
		return err
	}

	filtered := make([]OfflinePlaylist, 0, len(playlists))
	for _, playlist := range playlists {
		tracks := make([]api.Track, 0, len(playlist.Tracks))
		for _, track := range playlist.Tracks {
			if track.Id == "" || !existsIn(dir, track.Id) {
				continue
			}
			track.Available = true
			tracks = append(tracks, track)
		}
		if len(tracks) == 0 {
			continue
		}
		filtered = append(filtered, OfflinePlaylist{
			Name:   playlist.Name,
			Kind:   playlist.Kind,
			Tracks: tracks,
		})
	}
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })

	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}

	offlineMu.Lock()
	defer offlineMu.Unlock()

	path := filepath.Join(dir, offlineCatalogFile)
	tmp, err := os.CreateTemp(dir, "offline-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// LoadOfflineCatalog reads the catalog and drops the entries whose files are
// gone. A missing catalog is not an error: it just means nothing was saved
// for the offline mode yet.
func LoadOfflineCatalog() ([]OfflinePlaylist, error) {
	dir, err := getCacheDir()
	if err != nil {
		return nil, err
	}

	offlineMu.Lock()
	data, err := os.ReadFile(filepath.Join(dir, offlineCatalogFile))
	offlineMu.Unlock()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var playlists []OfflinePlaylist
	if err := json.Unmarshal(data, &playlists); err != nil {
		return nil, err
	}

	// The file may be old: check every track against the disk again, so the
	// offline list never offers a track that cannot be played.
	valid := playlists[:0]
	for _, playlist := range playlists {
		tracks := playlist.Tracks[:0]
		for _, track := range playlist.Tracks {
			if track.Id == "" || !existsIn(dir, track.Id) {
				continue
			}
			track.Available = true
			tracks = append(tracks, track)
		}
		if len(tracks) == 0 {
			continue
		}
		playlist.Tracks = tracks
		valid = append(valid, playlist)
	}
	return valid, nil
}

// existsIn reports whether the track file is present and not empty. The
// directory is resolved once by the caller: a playlist may have hundreds of
// tracks and the directory must not be created for each of them.
func existsIn(dir, trackID string) bool {
	stat, err := os.Stat(filepath.Join(dir, trackID+".mp3"))
	return err == nil && stat.Size() > 0
}

// PrunePartials removes the files of interrupted downloads. They were never
// committed, so they cannot be played; keeping them only wastes the disk and
// makes the cache look bigger than it is.
func PrunePartials() int {
	dir, err := getCacheDir()
	if err != nil {
		return 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".part" {
			continue
		}
		if os.Remove(filepath.Join(dir, entry.Name())) == nil {
			removed++
		}
	}
	return removed
}

// IntegrityCheck reports whether a cached track is present and not empty. It
// is the cheap check the offline list and the cache manager use; the decoder
// performs the real validation when the track starts.
func IntegrityCheck(trackID string) bool {
	if trackID == "" {
		return false
	}
	dir, err := getCacheDir()
	if err != nil {
		return false
	}
	stat, err := os.Stat(filepath.Join(dir, trackID+".mp3"))
	return err == nil && stat.Size() > 0
}
