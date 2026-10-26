//go:build !windows

package discordrpc

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

// dialDiscord tries the sockets Discord exposes on Unix.
func dialDiscord() (io.ReadWriteCloser, error) {
	var candidates []string
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		candidates = append(candidates, dir)
	}
	candidates = append(candidates,
		"/tmp",
		"/run/user/1000",
		filepath.Join(os.Getenv("HOME"), ".config/discord"),
	)

	var firstErr error
	for _, dir := range candidates {
		for i := 0; i < 10; i++ {
			path := filepath.Join(dir, fmt.Sprintf("discord-ipc-%d", i))
			conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
			if err == nil {
				return conn, nil
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return nil, firstErr
}
