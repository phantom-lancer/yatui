package mainpage

import (
	"os"
	"testing"
)

// TestMain isolates the package from the real user environment. Several tests
// save the config; without this, a test run rewrites the config of the person
// who runs it, including the token inside it.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "yatui-test-home-")
	if err != nil {
		panic(err)
	}

	os.Setenv("HOME", home)
	os.Setenv("USERPROFILE", home)
	os.Setenv("XDG_CONFIG_HOME", home+"/.config")
	os.Setenv("XDG_CACHE_HOME", home+"/.cache")
	os.Setenv("TMPDIR", home+"/tmp")
	os.MkdirAll(home+"/tmp", 0700)

	code := m.Run()

	os.RemoveAll(home)
	os.Exit(code)
}
