package updater

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current string
		latest  string
		want    bool
	}{
		{"v0.10.2", "v0.11.0", true},
		{"v0.10.2", "v0.10.3", true},
		{"v1.0.0", "v0.99.99", false},
		{"v1.2.3", "v1.2.3", false},
		{"v0.10.2", "0.10.2", false},
	}
	for _, test := range cases {
		got, err := IsNewer(test.current, test.latest)
		if err != nil {
			t.Fatalf("IsNewer(%q, %q): %v", test.current, test.latest, err)
		}
		if got != test.want {
			t.Fatalf("IsNewer(%q, %q) = %v, want %v", test.current, test.latest, got, test.want)
		}
	}
}

func TestIsNewerRejectsInvalidVersions(t *testing.T) {
	if _, err := IsNewer("not-a-version", "v0.11.0"); err == nil {
		t.Fatal("invalid current version was accepted")
	}
	if _, err := IsNewer("v0.10.2", "vNext"); err == nil {
		t.Fatal("invalid release version was accepted")
	}
}

func TestChecksumFor(t *testing.T) {
	data := []byte(strings.Repeat("a", 64) + "  yatui-windows-amd64.exe\n")
	got, err := checksumFor(data, "yatui-windows-amd64.exe")
	if err != nil {
		t.Fatal(err)
	}
	if got != strings.Repeat("a", 64) {
		t.Fatalf("checksum = %q", got)
	}
}

func TestChecksumForRejectsMissingAndMalformedEntries(t *testing.T) {
	if _, err := checksumFor([]byte(""), "yatui"); err == nil {
		t.Fatal("missing checksum was accepted")
	}
	if _, err := checksumFor([]byte("nope  yatui"), "yatui"); err == nil {
		t.Fatal("malformed checksum was accepted")
	}
}

type testRoundTripper func(*http.Request) (*http.Response, error)

func (f testRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestCheckLatestAtomBuildsReleaseAssets(t *testing.T) {
	const feed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>tag:github.com,2008:Repository/123/v0.12.0</id>
    <link rel="alternate" type="text/html" href="https://github.com/phantom-lancer/yatui/releases/tag/v0.12.0"/>
    <title>v0.12.0</title>
  </entry>
</feed>`
	client := &http.Client{Transport: testRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "github.com" || !strings.HasSuffix(request.URL.Path, "/releases.atom") {
			t.Fatalf("unexpected feed URL: %s", request.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(feed)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}

	release, err := checkLatestAtom(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if release.TagName != "v0.12.0" {
		t.Fatalf("tag = %q", release.TagName)
	}
	if release.HTMLURL != "https://github.com/phantom-lancer/yatui/releases/tag/v0.12.0" {
		t.Fatalf("release URL = %q", release.HTMLURL)
	}
	if _, err := findAsset(release, "SHA256SUMS"); err != nil {
		t.Fatal(err)
	}
	if _, err := findAsset(release, "yatui-windows-amd64.exe"); err != nil {
		t.Fatal(err)
	}
}
