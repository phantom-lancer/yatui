package updater

import (
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
