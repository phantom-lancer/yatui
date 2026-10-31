package main

import "testing"

// TestParseArgs checks the command line: --login forces the login page, the
// version flags print the version, and the flags may come in any order.
func TestParseArgs(t *testing.T) {
	cases := []struct {
		args        []string
		forceLogin  bool
		showVersion bool
	}{
		{args: nil},
		{args: []string{"--login"}, forceLogin: true},
		{args: []string{"-login"}, forceLogin: true},
		{args: []string{"--version"}, showVersion: true},
		{args: []string{"-v"}, showVersion: true},
		{args: []string{"version"}, showVersion: true},
		{args: []string{"--login", "--version"}, forceLogin: true, showVersion: true},
		{args: []string{"--unknown"}},
	}

	for _, c := range cases {
		forceLogin, showVersion := parseArgs(c.args)
		if forceLogin != c.forceLogin || showVersion != c.showVersion {
			t.Errorf("parseArgs(%q) = %v, %v; want %v, %v",
				c.args, forceLogin, showVersion, c.forceLogin, c.showVersion)
		}
	}
}
