package stream

import (
	"runtime/debug"
	"sync"
	"time"
)

const (
	// _RELEASE_MIN_SIZE is the smallest buffer worth returning to the system.
	_RELEASE_MIN_SIZE = 4 << 20
	// _RELEASE_INTERVAL keeps fast track skipping from running a garbage
	// collection for every track: the memory is given back at most this often.
	_RELEASE_INTERVAL = 20 * time.Second
)

var (
	releaseMux  sync.Mutex
	lastRelease time.Time
)

// releaseMemory gives the memory of a buffer that was just dropped back to the
// system. The player keeps the whole track in memory, so without this the
// process stays at the size of the biggest track played for a long time: the
// garbage collector frees the buffer but hands the pages out to the next track
// instead of returning them to the OS.
func releaseMemory(size int) {
	if size < _RELEASE_MIN_SIZE {
		return
	}

	releaseMux.Lock()
	if time.Since(lastRelease) < _RELEASE_INTERVAL {
		releaseMux.Unlock()
		return
	}
	lastRelease = time.Now()
	releaseMux.Unlock()

	go debug.FreeOSMemory()
}
