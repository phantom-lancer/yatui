//go:build windows

package discordrpc

import (
	"fmt"
	"io"

	"github.com/Microsoft/go-winio"
)

// dialDiscord tries the pipes Discord exposes on Windows.
func dialDiscord() (io.ReadWriteCloser, error) {
	var firstErr error
	for i := 0; i < 10; i++ {
		path := fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i)
		conn, err := winio.DialPipe(path, nil)
		if err == nil {
			return conn, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
}
