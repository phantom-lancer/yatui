package main

import (
	"strings"
	"testing"

	mainpage "github.com/phantom-lancer/yatui/ui/model/mainPage"
)

// TestParseArgs checks the command line: --login forces the login page, the
// version flags print the version, --offline and --doctor are independent and
// the flags may come in any order.
func TestParseArgs(t *testing.T) {
	cases := []struct {
		args     []string
		expected cliOptions
	}{
		{args: nil},
		{args: []string{"--login"}, expected: cliOptions{forceLogin: true}},
		{args: []string{"-login"}, expected: cliOptions{forceLogin: true}},
		{args: []string{"--version"}, expected: cliOptions{showVersion: true}},
		{args: []string{"-v"}, expected: cliOptions{showVersion: true}},
		{args: []string{"version"}, expected: cliOptions{showVersion: true}},
		{args: []string{"--login", "--version"}, expected: cliOptions{forceLogin: true, showVersion: true}},
		{args: []string{"--offline"}, expected: cliOptions{offline: true}},
		{args: []string{"offline"}, expected: cliOptions{offline: true}},
		{args: []string{"--doctor"}, expected: cliOptions{doctor: true}},
		{args: []string{"--doctor", "--json"}, expected: cliOptions{doctor: true, doctorJSON: true}},
		{args: []string{"--unknown"}},
	}

	for _, c := range cases {
		if got := parseArgs(c.args); got != c.expected {
			t.Errorf("parseArgs(%q) = %+v; want %+v", c.args, got, c.expected)
		}
	}
}

// TestBuildLabel checks that the sign under the splash always shows the
// version first, whatever the build metadata says.
func TestBuildLabel(t *testing.T) {
	label := buildLabel()
	if !strings.HasPrefix(label, mainpage.Version) {
		t.Fatalf("build label = %q, want the version %q first", label, mainpage.Version)
	}
}
