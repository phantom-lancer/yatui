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

func Write(trackId string) (*os.File, error) {
	dir, err := getCacheDir()
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(filepath.Join(dir, trackId+".mp3"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return nil, err
	}

	return file, nil
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
