package cache

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/phantom-lancer/yatui/config"
)

// The cache directory is resolved once and then guarded by its own lock: the
// download workers, the audio thread and the UI all ask for it, and reading
// config.Current from those goroutines was a data race. main calls SetDir
// after the config is loaded; without it the user cache directory is used,
// which keeps the package usable from tests.
var (
	dirMu   sync.RWMutex
	dirPath string
)

// SetDir selects the cache directory explicitly. An empty value restores the
// default location.
func SetDir(path string) {
	dirMu.Lock()
	dirPath = path
	dirMu.Unlock()
}

func configuredDir() string {
	dirMu.RLock()
	defer dirMu.RUnlock()
	return dirPath
}

func getCacheDir() (string, error) {
	var (
		cacheDir string
		err      error
	)

	if configured := configuredDir(); configured != "" {
		cacheDir, err = filepath.Abs(configured)
		if err != nil {
			return "", err
		}
	} else {
		userDir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		cacheDir = filepath.Join(userDir, config.DirName)
	}

	err = os.MkdirAll(cacheDir, 0700)
	if err != nil {
		return "", err
	}
	// Tracks and the loudness database are user data. Old versions created
	// the directory world readable; tighten it when it is opened.
	_ = os.Chmod(cacheDir, 0700)

	return cacheDir, nil
}

// Dir returns the directory the cached tracks are stored in.
func Dir() (string, error) {
	return getCacheDir()
}

func Read(trackId string) (*os.File, int64, error) {
	dir, err := getCacheDir()
	if err != nil {
		return nil, 0, err
	}

	file, err := os.Open(filepath.Join(dir, trackId+".mp3"))
	if err != nil {
		return nil, 0, err
	}

	stat, _ := file.Stat()
	return file, stat.Size(), nil
}

// Write creates a temporary file for the track. It is not visible in the
// cache until Commit renames it, so an interrupted write never leaves a
// truncated track that looks playable.
func Write(trackId string) (*os.File, error) {
	dir, err := getCacheDir()
	if err != nil {
		return nil, err
	}

	file, err := os.CreateTemp(dir, trackId+".*.part")
	if err != nil {
		return nil, err
	}
	_ = file.Chmod(0600)

	return file, nil
}

// Commit publishes a file created by Write. The data is flushed to the disk
// before the rename, so a crash cannot leave a half-written mp3 under the
// real name.
func Commit(file *os.File, trackId string) error {
	dir, err := getCacheDir()
	if err != nil {
		return err
	}

	tmpName := file.Name()
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(tmpName)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	_ = os.Chmod(tmpName, 0600)

	return os.Rename(tmpName, filepath.Join(dir, trackId+".mp3"))
}

// Discard removes a file created by Write and not committed.
func Discard(file *os.File) {
	name := file.Name()
	file.Close()
	os.Remove(name)
}

func Remove(trackId string) error {
	dir, err := getCacheDir()
	if err != nil {
		return err
	}

	return os.Remove(filepath.Join(dir, trackId+".mp3"))
}

// Exists reports whether the track is present in the cache.
func Exists(trackId string) bool {
	dir, err := getCacheDir()
	if err != nil {
		return false
	}

	_, err = os.Stat(filepath.Join(dir, trackId+".mp3"))
	return err == nil
}

// Size returns the size of the cached track.
func Size(trackId string) (int64, error) {
	dir, err := getCacheDir()
	if err != nil {
		return 0, err
	}
	stat, err := os.Stat(filepath.Join(dir, trackId+".mp3"))
	if err != nil {
		return 0, err
	}
	return stat.Size(), nil
}
