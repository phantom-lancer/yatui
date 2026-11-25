package log

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogWritesAndFlushesOnStop(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)

	Start()
	path := Location()
	if path == "" {
		t.Fatal("the log file was not created")
	}
	Print(LVL_INFO, "hello %s", "world")
	Stop()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %s", err)
	}
	if !strings.Contains(string(data), "hello world") {
		t.Fatalf("the line was lost: %q", data)
	}
	if !strings.Contains(string(data), "[ INFO ]") {
		t.Fatalf("the level is missing: %q", data)
	}

	if runtime.GOOS != "windows" {
		stat, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := stat.Mode().Perm(); perm != 0600 {
			t.Fatalf("the log is %04o, want 0600", perm)
		}
	}
}

func TestLogStopIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)

	Start()
	Stop()
	Stop()
	Print(LVL_INFO, "after stop")

	if Location() != "" {
		t.Fatal("a stopped log still reports a location")
	}
}

func TestLogConcurrentWritersDoNotBlock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)

	Start()
	defer Stop()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				Print(LVL_INFO, "writer %d line %d", n, j)
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent logging blocked")
	}
}

func TestLogFlushTimerWritesWithoutStop(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)

	Start()
	defer Stop()
	Print(LVL_INFO, "timer line")

	path := Location()
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, err := os.ReadFile(filepath.Clean(path))
		if err == nil && strings.Contains(string(data), "timer line") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the timer did not flush the line: %q", data)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
