package cache

import (
	"os"
	"path/filepath"

	"github.com/phantom-lancer/yatui/config"
)

func getCacheDir() (string, error) {
	var (
		cacheDir string
		err      error
	)

	if len(config.Current.CacheDir) == 0 {
		userDir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		cacheDir = filepath.Join(userDir, config.DirName)
	} else {
		cacheDir, err = filepath.Abs(config.Current.CacheDir)
		if err != nil {
			return "", err
		}
	}

	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return "", err
	}

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
