// Package discordrpc speaks the local Discord IPC protocol directly.
//
// It is a small implementation of what PulseSync uses: the same activity
// fields, the same two links and the same playing/paused icons. Only the
// transport and the JSON framing live here.
package discordrpc

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

// opcodes of the IPC protocol.
const (
	opHandshake = 0
	opFrame     = 1
)

// Activity is one rich presence payload.
type Activity struct {
	Details           string      `json:"details,omitempty"`
	State             string      `json:"state,omitempty"`
	DetailsURL        string      `json:"details_url,omitempty"`
	StateURL          string      `json:"state_url,omitempty"`
	Type              int         `json:"type"`
	StatusDisplayType int         `json:"status_display_type"`
	Timestamps        *Timestamps `json:"timestamps,omitempty"`
	Assets            *Assets     `json:"assets,omitempty"`
	Buttons           []Button    `json:"buttons,omitempty"`
	Instance          bool        `json:"instance"`
}

// Timestamps is the start/end pair of the listening activity.
type Timestamps struct {
	Start int64 `json:"start,omitempty"`
	End   int64 `json:"end,omitempty"`
}

// Assets are the images of the activity.
type Assets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
	LargeURL   string `json:"large_url,omitempty"`
	SmallImage string `json:"small_image,omitempty"`
	SmallText  string `json:"small_text,omitempty"`
	SmallURL   string `json:"small_url,omitempty"`
}

// Button is one clickable link under the activity.
type Button struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Client is a connected Discord IPC client.
type Client struct {
	mu       sync.Mutex
	conn     io.ReadWriteCloser
	clientID string
	nonce    int
	ready    chan struct{}
	readyOne sync.Once
	dead     bool
}

// Dial connects to the Discord client and waits until it says READY.
func Dial(clientID string) (*Client, error) {
	conn, err := dialDiscord()
	if err != nil {
		return nil, err
	}

	c := &Client{conn: conn, clientID: clientID, ready: make(chan struct{})}

	if err := c.send(opHandshake, map[string]interface{}{"v": 1, "client_id": clientID}); err != nil {
		conn.Close()
		return nil, err
	}

	go c.readLoop()

	select {
	case <-c.ready:
		return c, nil
	case <-time.After(3 * time.Second):
		conn.Close()
		return nil, errors.New("discordrpc: handshake timeout")
	}
}

// readLoop drains the responses: Discord stops accepting writes when nobody
// reads the pipe, which is exactly what caused i/o timeouts before.
func (c *Client) readLoop() {
	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(c.conn, header); err != nil {
			c.mu.Lock()
			c.dead = true
			c.mu.Unlock()
			c.readyOne.Do(func() { close(c.ready) })
			return
		}

		size := int(binary.LittleEndian.Uint32(header[4:8]))
		if size < 0 || size > 1<<20 {
			continue
		}
		body := make([]byte, size)
		if _, err := io.ReadFull(c.conn, body); err != nil {
			c.mu.Lock()
			c.dead = true
			c.mu.Unlock()
			return
		}

		var frame struct {
			Evt string `json:"evt"`
		}
		if json.Unmarshal(body, &frame) == nil && frame.Evt == "READY" {
			c.readyOne.Do(func() { close(c.ready) })
		}
	}
}

// Dead reports whether the pipe is already closed by Discord.
func (c *Client) Dead() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dead || c.conn == nil
}

// SetActivity replaces the presence of the current process.
func (c *Client) SetActivity(activity Activity) error {
	return c.command(map[string]interface{}{
		"pid":      os.Getpid(),
		"activity": activity,
	})
}

// Clear removes the presence of the current process.
func (c *Client) Clear() error {
	return c.command(map[string]interface{}{"pid": os.Getpid()})
}

func (c *Client) command(args map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.dead {
		return errors.New("discordrpc: connection closed")
	}

	c.nonce++
	return c.sendLocked(opFrame, map[string]interface{}{
		"cmd":   "SET_ACTIVITY",
		"args":  args,
		"nonce": c.nonce,
	})
}

// Close closes the pipe.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	c.dead = true
	err := c.conn.Close()
	c.conn = nil
	return err
}

func (c *Client) send(opcode int, payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendLocked(opcode, payload)
}

func (c *Client) sendLocked(opcode int, payload interface{}) error {
	if c.conn == nil {
		return errors.New("discordrpc: not connected")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	frame := make([]byte, 8+len(body))
	binary.LittleEndian.PutUint32(frame[0:4], uint32(opcode))
	binary.LittleEndian.PutUint32(frame[4:8], uint32(len(body)))
	copy(frame[8:], body)

	if setter, ok := c.conn.(interface{ SetWriteDeadline(time.Time) error }); ok {
		_ = setter.SetWriteDeadline(time.Now().Add(5 * time.Second))
	}
	_, err = c.conn.Write(frame)
	return err
}
