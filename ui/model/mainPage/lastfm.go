package mainpage

import (
	"os/exec"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/lastfm"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/ui/helpers"
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

// lastfmPollMsg asks again whether the user granted access in the browser.
type lastfmPollMsg struct {
	token   string
	attempt int
}

// lastfmPollCmd waits and checks the token once more. The whole web flow is
// one click for the user: browser opens, they press "Allow", the app notices.
func lastfmPollCmd(token string, attempt int) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(3 * time.Second)
		key, secret := lastfmCredentials()
		client := lastfm.New(key, secret, "")
		sessionKey, user, err := client.GetSession(token)
		if err != nil {
			if attempt >= 60 {
				return lastfmSessionMsg{err: err}
			}
			return lastfmPollMsg{token: token, attempt: attempt + 1}
		}
		return lastfmSessionMsg{key: sessionKey, user: user}
	}
}

// lastfmCredentials returns the keys used for the requests: the user's own
// ones if present, otherwise the app keys built into the binary.
func lastfmCredentials() (string, string) {
	cfg := config.Current.LastFM
	if cfg == nil {
		return lastfm.Credentials("", "")
	}
	return lastfm.Credentials(cfg.ApiKey, cfg.ApiSecret)
}

// lastfmConfigured reports whether the api key and secret are in place.
func lastfmConfigured() bool {
	key, secret := lastfmCredentials()
	return key != "" && secret != ""
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
		return "нет ключей"
	case cfg.SessionKey == "":
		return "войти"
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

	key, secret := lastfmCredentials()
	if key == "" || secret == "" {
		m.showToast("Last.fm: в сборке нет ключей приложения")
		return nil
	}

	if cfg.SessionKey != "" {
		cfg.Enabled = !cfg.Enabled
		m.saveConfig()
		m.showToast("Last.fm: " + onOff(cfg.Enabled))
		return nil
	}

	client := lastfm.New(key, secret, "")

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
func (m *Model) handleLastfmMsg(message tea.Msg) (tea.Cmd, bool) {
	cfg := config.Current.LastFM
	if cfg == nil {
		return nil, false
	}

	switch msg := message.(type) {
	case lastfmTokenMsg:
		if msg.err != nil {
			m.showToast("Last.fm: " + msg.err.Error())
			return nil, true
		}
		cfg.PendingToken = msg.token
		m.saveConfig()
		key, _ := lastfmCredentials()
		openBrowser(lastfm.AuthUrl(key, msg.token))
		m.showToast("Last.fm: разреши доступ в браузере — подключусь сам")
		return lastfmPollCmd(msg.token, 0), true

	case lastfmPollMsg:
		return lastfmPollCmd(msg.token, msg.attempt), true

	case lastfmSessionMsg:
		if msg.err != nil {
			m.showToast("Last.fm: " + msg.err.Error())
			return nil, true
		}
		applyLastfmSession(cfg, msg.key, msg.user)
		m.saveConfig()
		m.showToast("Last.fm подключён: " + msg.user)
		return nil, true
	}

	return nil, false
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
	key, secret := lastfmCredentials()
	client := lastfmClient(key, secret, cfg.SessionKey)

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
