package mainpage

import (
	"errors"
	"testing"
	"time"

	"github.com/dece2183/yamusic-tui/api"
	"github.com/dece2183/yamusic-tui/config"
)

// fakeScrobbler records the calls instead of talking to Last.fm.
type fakeScrobbler struct {
	nowPlaying chan string
	scrobbles  chan string
}

func newFakeScrobbler() *fakeScrobbler {
	return &fakeScrobbler{
		nowPlaying: make(chan string, 4),
		scrobbles:  make(chan string, 4),
	}
}

func (f *fakeScrobbler) NowPlaying(artist, title, album string, duration int) error {
	f.nowPlaying <- artist + " - " + title
	return nil
}

func (f *fakeScrobbler) Scrobble(artist, title, album string, timestamp int64, duration int) error {
	f.scrobbles <- artist + " - " + title
	return nil
}

func withLastfm(t *testing.T, cfg *config.LastFM) {
	t.Helper()
	previous := config.Current.LastFM
	config.Current.LastFM = cfg
	t.Cleanup(func() { config.Current.LastFM = previous })
}

func TestLastfmStatus(t *testing.T) {
	cases := []struct {
		cfg  config.LastFM
		want string
	}{
		{config.LastFM{}, "нужен api-key"},
		{config.LastFM{ApiKey: "k", ApiSecret: "s"}, "подключить"},
		{config.LastFM{ApiKey: "k", ApiSecret: "s", SessionKey: "sk", Username: "user", Enabled: true}, "user вкл"},
		{config.LastFM{ApiKey: "k", ApiSecret: "s", SessionKey: "sk", Username: "user"}, "user выкл"},
	}

	for _, c := range cases {
		cfg := c.cfg
		withLastfm(t, &cfg)

		m := &Model{}
		if got := m.lastfmStatus(); got != c.want {
			t.Fatalf("status = %q, want %q", got, c.want)
		}
	}
}

func TestScrobbleInactiveSchedulesNothing(t *testing.T) {
	withLastfm(t, &config.LastFM{ApiKey: "k", ApiSecret: "s", SessionKey: "sk"})
	// Enabled is false: nothing must be sent

	m := &Model{}
	m.scrobbleStart(&api.Track{Title: "T", DurationMs: 180000})

	if m.scrobbleGen != 0 {
		t.Fatalf("generation changed: %d", m.scrobbleGen)
	}
}

func TestScrobbleSendsNowPlaying(t *testing.T) {
	withLastfm(t, &config.LastFM{ApiKey: "k", ApiSecret: "s", SessionKey: "sk", Enabled: true})

	fake := newFakeScrobbler()
	previousClient := lastfmClient
	lastfmClient = func(apiKey, apiSecret, sessionKey string) scrobbler { return fake }
	t.Cleanup(func() { lastfmClient = previousClient })

	m := &Model{}
	track := &api.Track{Title: "Title", DurationMs: 20000}
	track.Artists = []api.Artist{{Name: "Artist"}}

	m.scrobbleStart(track)
	if m.scrobbleGen != 1 {
		t.Fatalf("generation = %d, want 1", m.scrobbleGen)
	}

	select {
	case got := <-fake.nowPlaying:
		if got != "Artist - Title" {
			t.Fatalf("now playing = %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("now playing was not sent")
	}

	// a track shorter than 30 seconds is never scrobbled
	select {
	case got := <-fake.scrobbles:
		t.Fatalf("short track was scrobbled: %q", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestScrobbleCancelDropsPendingTrack(t *testing.T) {
	withLastfm(t, &config.LastFM{ApiKey: "k", ApiSecret: "s", SessionKey: "sk", Enabled: true})

	fake := newFakeScrobbler()
	previousClient := lastfmClient
	lastfmClient = func(apiKey, apiSecret, sessionKey string) scrobbler { return fake }
	t.Cleanup(func() { lastfmClient = previousClient })

	m := &Model{}
	m.scrobbleStart(&api.Track{Title: "T", DurationMs: 180000})
	generation := m.scrobbleGen

	m.scrobbleCancel()
	if m.scrobbleGen == generation {
		t.Fatal("cancel must invalidate the pending scrobble")
	}
}

func TestLastfmSetupNeedsCredentials(t *testing.T) {
	withLastfm(t, &config.LastFM{})

	m := &Model{}
	if cmd := m.lastfmSetupCmd(); cmd != nil {
		t.Fatal("no command without an api key")
	}
}

func TestLastfmSetupStartsAuth(t *testing.T) {
	withLastfm(t, &config.LastFM{ApiKey: "k", ApiSecret: "s"})

	m := &Model{}
	cmd := m.lastfmSetupCmd()
	if cmd == nil {
		t.Fatal("auth command is missing")
	}
}

func TestHandleLastfmError(t *testing.T) {
	withLastfm(t, &config.LastFM{ApiKey: "k", ApiSecret: "s"})

	m := &Model{}
	if !m.handleLastfmMsg(lastfmTokenMsg{err: errors.New("boom")}) {
		t.Fatal("token errors must be handled")
	}
	if !m.handleLastfmMsg(lastfmSessionMsg{err: errors.New("boom")}) {
		t.Fatal("session errors must be handled")
	}
	if m.handleLastfmMsg("not a lastfm message") {
		t.Fatal("unrelated messages must be ignored")
	}
}

func TestApplyLastfmSession(t *testing.T) {
	cfg := &config.LastFM{ApiKey: "k", ApiSecret: "s", PendingToken: "token"}

	applyLastfmSession(cfg, "sk", "user")

	if cfg.SessionKey != "sk" || cfg.Username != "user" || cfg.PendingToken != "" || !cfg.Enabled {
		t.Fatalf("unexpected config: %+v", cfg)
	}

	if got := lastfmStatusOf(cfg); got != "user вкл" {
		t.Fatalf("status = %q", got)
	}
}

// lastfmStatusOf mirrors the settings row without touching the live config.
func lastfmStatusOf(cfg *config.LastFM) string {
	m := &Model{}
	previous := config.Current.LastFM
	config.Current.LastFM = cfg
	defer func() { config.Current.LastFM = previous }()
	return m.lastfmStatus()
}
