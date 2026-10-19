package mainpage

import (
	"os/exec"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dece2183/yamusic-tui/api"
	"github.com/dece2183/yamusic-tui/config"
	"github.com/dece2183/yamusic-tui/lastfm"
	"github.com/dece2183/yamusic-tui/log"
	"github.com/dece2183/yamusic-tui/ui/helpers"
)

const (
	// scrobbleMinDuration is the shortest track Last.fm accepts.
	scrobbleMinDuration = 30 * time.Second
	// scrobbleAfter is the longest wait before a track is scrobbled.
	scrobbleAfter = 4 * time.Minute
)

type lastfmTokenMsg struct {
	token string
	err   error
}

type lastfmSessionMsg struct {
	key  string
	user string
	err  error
}

// lastfmConfigured reports whether the api key and secret are in place.
func lastfmConfigured() bool {
	cfg := config.Current.LastFM
	return cfg != nil && cfg.ApiKey != "" && cfg.ApiSecret != ""
}

// lastfmActive reports whether scrobbling can actually be sent.
func lastfmActive() bool {
	cfg := config.Current.LastFM
	return lastfmConfigured() && cfg.Enabled && cfg.SessionKey != ""
}

// lastfmStatus is the value shown in the settings row.
func (m *Model) lastfmStatus() string {
	cfg := config.Current.LastFM
	switch {
	case !lastfmConfigured():
		return "нужен api-key"
	case cfg.SessionKey == "":
		return "подключить"
	case cfg.Enabled:
		return cfg.Username + " вкл"
	default:
		return cfg.Username + " выкл"
	}
}

// lastfmSetupCmd runs the next step of the connection flow:
// nothing configured -> hint, token pending -> finish, otherwise -> toggle
// (or start the auth flow on the first run).
func (m *Model) lastfmSetupCmd() tea.Cmd {
	cfg := config.Current.LastFM
	if cfg == nil {
		return nil
	}

	if cfg.ApiKey == "" || cfg.ApiSecret == "" {
		m.showToast("Last.fm: впиши api-key и api-secret в config.yaml")
		return nil
	}

	if cfg.SessionKey != "" {
		cfg.Enabled = !cfg.Enabled
		m.saveConfig()
		m.showToast("Last.fm: " + onOff(cfg.Enabled))
		return nil
	}

	client := lastfm.New(cfg.ApiKey, cfg.ApiSecret, "")

	if cfg.PendingToken != "" {
		token := cfg.PendingToken
		m.showToast("Last.fm: проверяю доступ…")
		return func() tea.Msg {
			key, user, err := client.GetSession(token)
			return lastfmSessionMsg{key: key, user: user, err: err}
		}
	}

	return func() tea.Msg {
		token, err := client.GetToken()
		return lastfmTokenMsg{token: token, err: err}
	}
}

// handleLastfmMsg processes the async auth results.
func (m *Model) handleLastfmMsg(message tea.Msg) bool {
	cfg := config.Current.LastFM
	if cfg == nil {
		return false
	}

	switch msg := message.(type) {
	case lastfmTokenMsg:
		if msg.err != nil {
			m.showToast("Last.fm: " + msg.err.Error())
			return true
		}
		cfg.PendingToken = msg.token
		m.saveConfig()
		openBrowser(lastfm.AuthUrl(cfg.ApiKey, msg.token))
		m.showToast("Last.fm: разреши доступ в браузере, потом F9 → Last.fm")
		return true

	case lastfmSessionMsg:
		if msg.err != nil {
			m.showToast("Last.fm: " + msg.err.Error())
			return true
		}
		applyLastfmSession(cfg, msg.key, msg.user)
		m.saveConfig()
		m.showToast("Last.fm подключён: " + msg.user)
		return true
	}

	return false
}

// applyLastfmSession stores the result of the web auth flow.
func applyLastfmSession(cfg *config.LastFM, key, user string) {
	cfg.SessionKey = key
	cfg.Username = user
	cfg.PendingToken = ""
	cfg.Enabled = true
}

// openBrowser opens the auth page in the default browser. A failure is
// harmless: the user can open the printed url manually.
func openBrowser(link string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	if err := cmd.Start(); err != nil {
		log.Print(log.LVL_WARNIGN, "failed to open the browser: %s", err)
	}
}

// scrobbler is the part of the Last.fm client the player uses. It is an
// interface so the tests can replace the network with a recorder.
type scrobbler interface {
	NowPlaying(artist, title, album string, duration int) error
	Scrobble(artist, title, album string, timestamp int64, duration int) error
}

// lastfmClient is a variable to keep the network out of the tests.
var lastfmClient = func(apiKey, apiSecret, sessionKey string) scrobbler {
	return lastfm.New(apiKey, apiSecret, sessionKey)
}

// scrobbleStart reports the now playing track and schedules the scrobble.
// The timer is a plain sleeping goroutine, so an idle player schedules
// nothing at all.
func (m *Model) scrobbleStart(track *api.Track) {
	if track == nil || !lastfmActive() {
		return
	}

	cfg := config.Current.LastFM
	client := lastfmClient(cfg.ApiKey, cfg.ApiSecret, cfg.SessionKey)

	artist := helpers.ArtistList(track.Artists)
	title := track.Title
	album := ""
	if len(track.Albums) > 0 {
		album = track.Albums[0].Title
	}
	duration := track.DurationMs / 1000

	// a new track invalidates the pending scrobble of the previous one
	generation := atomic.AddInt64(&m.scrobbleGen, 1)
	scrobbledAt := time.Now().Unix()

	go func() {
		if err := client.NowPlaying(artist, title, album, duration); err != nil {
			log.Print(log.LVL_WARNIGN, "lastfm now playing: %s", err)
		}
	}()

	if track.DurationMs <= 0 || track.DurationMs < int(scrobbleMinDuration/time.Millisecond) {
		return
	}

	delay := time.Duration(track.DurationMs/2) * time.Millisecond
	if delay > scrobbleAfter {
		delay = scrobbleAfter
	}

	go func() {
		time.Sleep(delay)
		if atomic.LoadInt64(&m.scrobbleGen) != generation {
			return
		}
		if err := client.Scrobble(artist, title, album, scrobbledAt, duration); err != nil {
			log.Print(log.LVL_WARNIGN, "lastfm scrobble: %s", err)
		}
	}()
}

// scrobbleCancel drops the pending scrobble, e.g. when playback is stopped.
func (m *Model) scrobbleCancel() {
	atomic.AddInt64(&m.scrobbleGen, 1)
}
