// Package lastfm is a minimal Last.fm API client: only the three calls the
// player needs (now playing, scrobble and the web auth flow) are implemented.
// It uses the standard library only.
package lastfm

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	apiUrl    = "https://ws.audioscrobbler.com/2.0/"
	authUrl   = "https://www.last.fm/api/auth/"
	userAgent = "yamusic-tui"
)

// App credentials are registered once by the project, for all users. Users
// only sign in: they never see api keys. Fill these two values once on
// https://www.last.fm/api/account/create and commit them.
const (
	DefaultApiKey    = "703ed501ce5ecc7290cfca22ceda30cd"
	DefaultApiSecret = "78ba2c9bea4d0be783393acf1a1a75e0"
)

// Credentials returns the user's own keys if both are set, otherwise the
// built-in app keys.
func Credentials(apiKey, apiSecret string) (string, string) {
	if apiKey != "" && apiSecret != "" {
		return apiKey, apiSecret
	}
	return DefaultApiKey, DefaultApiSecret
}

// Client is bound to a single Last.fm account once the session key is known.
type Client struct {
	apiKey     string
	apiSecret  string
	sessionKey string
	http       *http.Client
}

func New(apiKey, apiSecret, sessionKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		sessionKey: sessionKey,
		http:       &http.Client{Timeout: 8 * time.Second},
	}
}

// AuthUrl returns the page the user has to open to grant access.
func AuthUrl(apiKey, token string) string {
	return authUrl + "?api_key=" + url.QueryEscape(apiKey) + "&token=" + url.QueryEscape(token)
}

// sign builds the API signature: md5 of the alphabetically sorted
// "namevalue" pairs followed by the shared secret.
func sign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString(params[key])
	}
	builder.WriteString(secret)

	sum := md5.Sum([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

// call performs a signed request and decodes the JSON response.
func (c *Client) call(method string, params map[string]string) (map[string]interface{}, error) {
	form := url.Values{}
	form.Set("method", method)
	form.Set("api_key", c.apiKey)
	form.Set("format", "json")

	signed := make(map[string]string, len(params)+2)
	signed["method"] = method
	signed["api_key"] = c.apiKey
	for key, value := range params {
		if value == "" {
			continue
		}
		form.Set(key, value)
		signed[key] = value
	}
	form.Set("api_sig", sign(signed, c.apiSecret))

	response, err := c.http.PostForm(apiUrl, form)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("lastfm: unexpected response: %w", err)
	}

	if message, ok := payload["error"]; ok {
		return nil, fmt.Errorf("lastfm: %v", message)
	}

	return payload, nil
}

// NowPlaying reports the track that just started.
func (c *Client) NowPlaying(artist, title, album string, duration int) error {
	if c.sessionKey == "" {
		return errors.New("lastfm: no session key")
	}

	_, err := c.call("track.updateNowPlaying", map[string]string{
		"artist":   artist,
		"track":    title,
		"album":    album,
		"duration": strconv.Itoa(duration),
		"sk":       c.sessionKey,
	})
	return err
}

// Scrobble reports a finished (or long enough) track.
func (c *Client) Scrobble(artist, title, album string, timestamp int64, duration int) error {
	if c.sessionKey == "" {
		return errors.New("lastfm: no session key")
	}

	_, err := c.call("track.scrobble", map[string]string{
		"artist":    artist,
		"track":     title,
		"album":     album,
		"timestamp": strconv.FormatInt(timestamp, 10),
		"duration":  strconv.Itoa(duration),
		"sk":        c.sessionKey,
	})
	return err
}

// GetToken starts the web auth flow.
func (c *Client) GetToken() (string, error) {
	payload, err := c.call("auth.getToken", nil)
	if err != nil {
		return "", err
	}

	token, ok := payload["token"].(string)
	if !ok || token == "" {
		return "", errors.New("lastfm: empty token")
	}
	return token, nil
}

// GetSession finishes the web auth flow and returns the session key and the
// user name.
func (c *Client) GetSession(token string) (string, string, error) {
	payload, err := c.call("auth.getSession", map[string]string{"token": token})
	if err != nil {
		return "", "", err
	}

	session, ok := payload["session"].(map[string]interface{})
	if !ok {
		return "", "", errors.New("lastfm: empty session")
	}

	key, _ := session["key"].(string)
	name, _ := session["name"].(string)
	if key == "" {
		return "", "", errors.New("lastfm: empty session key")
	}
	return key, name, nil
}
