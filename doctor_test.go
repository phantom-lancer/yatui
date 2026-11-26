package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phantom-lancer/yatui/cache"
	"github.com/phantom-lancer/yatui/config"
)

// isolateDoctor points the config and the cache at an empty temporary home.
func isolateDoctor(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("TMPDIR", filepath.Join(home, "tmp"))
	os.MkdirAll(filepath.Join(home, "tmp"), 0700)
	cache.SetDir("")
	t.Cleanup(func() { cache.SetDir("") })
}

// TestDoctorReportIsSanitized checks that the report describes the state
// without ever copying the token, the keys or the proxy address.
func TestDoctorReportIsSanitized(t *testing.T) {
	isolateDoctor(t)

	dir, err := config.Dir()
	if err != nil {
		t.Fatalf("config dir: %s", err)
	}
	secret := "y0__super-secret-token"
	content := "token: " + secret + "\n" +
		"lastfm:\n  enabled: false\n  api-key: " + secret + "\n" +
		"proxy: http://user:pass@example.test:8080\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0600); err != nil {
		t.Fatalf("write config: %s", err)
	}

	report := buildDoctorReport()
	if !report.Config.Exists || !report.Config.Parsed {
		t.Fatalf("the config was not recognized: %+v", report.Config)
	}
	if !report.Config.TokenSaved {
		t.Error("a saved token was not reported")
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %s", err)
	}
	text := string(data)
	for _, leaked := range []string{secret, "user:pass", "example.test"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("the report leaks %q", leaked)
		}
	}
}

// TestDoctorFindsBrokenConfig checks that a broken config is reported as an
// issue instead of crashing the diagnostic run.
func TestDoctorFindsBrokenConfig(t *testing.T) {
	isolateDoctor(t)

	dir, err := config.Dir()
	if err != nil {
		t.Fatalf("config dir: %s", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("token: [unclosed"), 0600); err != nil {
		t.Fatalf("write config: %s", err)
	}

	report := buildDoctorReport()
	if report.OK {
		t.Fatal("a broken config was reported as healthy")
	}
	if report.Config.Parsed {
		t.Fatal("a broken config was reported as parsed")
	}
	if len(report.Issues) == 0 {
		t.Fatal("no issue was reported")
	}
}

// TestDoctorDoesNotRewriteTheConfig checks the promise of the flag: a
// diagnostic run leaves the file byte for byte as it was.
func TestDoctorDoesNotRewriteTheConfig(t *testing.T) {
	isolateDoctor(t)

	dir, err := config.Dir()
	if err != nil {
		t.Fatalf("config dir: %s", err)
	}
	path := filepath.Join(dir, "config.yaml")
	// an old config without any of the new keys
	original := []byte("token: abc\nvolume: 0.3\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatalf("write config: %s", err)
	}

	buildDoctorReport()

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %s", err)
	}
	if string(after) != string(original) {
		t.Fatalf("the doctor rewrote the config:\n%s", after)
	}
}
