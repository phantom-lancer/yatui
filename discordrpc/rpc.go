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
}

// Dial connects to the Discord client and performs the handshake.
func Dial(clientID string) (*Client, error) {
	conn, err := dialDiscord()
	if err != nil {
		return nil, err
	}

	c := &Client{conn: conn, clientID: clientID}

	if err := c.send(0, map[string]interface{}{"v": 1, "client_id": clientID}); err != nil {
		conn.Close()
		return nil, err
	}

	return c, nil
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

	c.nonce++
	return c.send(1, map[string]interface{}{
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
	err := c.conn.Close()
	c.conn = nil
	return err
}

func (c *Client) send(opcode int, payload interface{}) error {
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
		_ = setter.SetWriteDeadline(time.Now().Add(2 * time.Second))
	}
	_, err = c.conn.Write(frame)
	return err
}
