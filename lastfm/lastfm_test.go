package lastfm

import "testing"

// The expected digest was computed independently (PowerShell MD5 of
// "api_keykeyartistAmethodtest.methodtrackTsec").
func TestSign(t *testing.T) {
	params := map[string]string{
		"artist": "A",
		"track":  "T",
	}
	params["method"] = "test.method"
	params["api_key"] = "key"

	got := sign(params, "sec")
	want := "67c24a690bcf34d1504e6b9b7f491ba2"
	if got != want {
		t.Fatalf("signature = %s, want %s", got, want)
	}
}

func TestAuthUrl(t *testing.T) {
	url := AuthUrl("my key", "tok en")
	if want := "https://www.last.fm/api/auth/?api_key=my+key&token=tok+en"; url != want {
		t.Fatalf("url = %q, want %q", url, want)
	}
}

func TestClientWithoutSession(t *testing.T) {
	c := New("key", "secret", "")

	if err := c.NowPlaying("artist", "title", "album", 100); err == nil {
		t.Fatal("now playing must fail without a session")
	}
	if err := c.Scrobble("artist", "title", "album", 0, 100); err == nil {
		t.Fatal("scrobble must fail without a session")
	}
}
