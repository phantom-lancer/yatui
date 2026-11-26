package log

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/phantom-lancer/yatui/config"
)

type Level int

const (
	LVL_PANIC Level = iota
	LVL_ERROR
	LVL_WARNIGN
	LVL_INFO
)

var lvlName = map[Level]string{
	LVL_PANIC:   "PANIC",
	LVL_ERROR:   "ERROR",
	LVL_WARNIGN: "WARN",
	LVL_INFO:    "INFO",
}

// The log is written through a small buffer. A log line used to call
// file.Sync, which writes to the disk on every line; on a slow disk that is a
// visible stall in the UI. The buffer is flushed by a timer and on Stop, and a
// mutex keeps concurrent writers from interleaving. A failing write disables
// logging instead of blocking the application.
var (
	mu     sync.Mutex
	file   *os.File
	writer *bufio.Writer
	stop   chan struct{}
	done   chan struct{}
	path   string
)

const (
	flushPeriod = time.Second
	// dropThreshold is the point where the buffer is no longer allowed to
	// grow: the timer flushes it, and a full buffer drops new lines instead
	// of blocking the audio or UI thread on the disk.
	dropThreshold = 256 << 10
)

func getLogLocation() (string, error) {
	tempDir := filepath.Join(os.TempDir(), config.DirName)
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return "", err
	}
	_ = os.Chmod(tempDir, 0700)
	return filepath.Join(tempDir, config.DirName+".log"), nil
}

// Location returns the log file path, or an empty string while logging is off.
func Location() string {
	mu.Lock()
	defer mu.Unlock()
	return path
}

func Start() {
	// a second call must not leave the timer of the first one running
	Stop()

	logPath, err := getLogLocation()
	if err != nil {
		return
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	_ = f.Chmod(0600)

	mu.Lock()
	if file != nil {
		// Start is called once, but a second call must not leak the first
		// writer.
		_ = file.Close()
	}
	file = f
	writer = bufio.NewWriterSize(f, 16<<10)
	path = logPath
	stop = make(chan struct{})
	done = make(chan struct{})
	stopCh, doneCh := stop, done
	mu.Unlock()

	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(flushPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				flush()
			case <-stopCh:
				return
			}
		}
	}()
}

func Stop() {
	mu.Lock()
	if file == nil {
		mu.Unlock()
		return
	}
	stopCh, doneCh := stop, done
	stop, done = nil, nil
	mu.Unlock()

	if stopCh != nil {
		close(stopCh)
		<-doneCh
	}

	flush()

	mu.Lock()
	if writer != nil {
		_ = writer.Flush()
		writer = nil
	}
	if file != nil {
		_ = file.Close()
		file = nil
	}
	path = ""
	mu.Unlock()
}

func flush() {
	mu.Lock()
	defer mu.Unlock()
	if writer != nil {
		_ = writer.Flush()
	}
}

// Print writes one formatted line. It never blocks on the audio or UI thread:
// the line goes into an in-memory buffer unless the buffer is already full.
func Print(lvl Level, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()

	if writer == nil {
		return
	}

	t := time.Now()
	format = fmt.Sprintf("[ %s ][(%d) %02d:%02d:%02d] - ", lvlName[lvl], t.Day(), t.Hour(), t.Minute(), t.Second()) + format + "\n"
	if writer.Buffered() >= dropThreshold {
		// The disk cannot keep up. Dropping a line is better than stalling
		// the caller, which may be the audio callback.
		return
	}
	if _, err := fmt.Fprintf(writer, format, args...); err != nil {
		// Avoid repeated write attempts if the disk is gone.
		return
	}

}
