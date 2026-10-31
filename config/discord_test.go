package config

import "testing"

// TestDiscordDefaults guards the settings of a fresh config: the card must be
// compact, the status is the artist, and the card is hidden five minutes after
// the pause.
func TestDiscordDefaults(t *testing.T) {
	d := DefaultDiscord()

	if !d.Enabled {
		t.Error("the Discord card must be on")
	}
	if d.StatusType != 1 {
		t.Errorf("the status must show the artist (1), got %d", d.StatusType)
	}
	if d.Language != "en" {
		t.Errorf("the status language is %q", d.Language)
	}
	if d.ShowAlbum || d.ShowVersion || d.ShowSmallIcon || d.ShowButtons || d.ShowBranding {
		t.Errorf("the card must stay compact, got %+v", d)
	}
	if d.MergeLinks {
		t.Error("the links must stay separate")
	}
	if d.AfkTimeout != 5 {
		t.Errorf("the pause timeout is %d minutes", d.AfkTimeout)
	}
}

// TestLegacyDiscordDiffers checks that the old defaults are recognizable: a
// config that matches them is migrated once, a tuned one is kept.
func TestLegacyDiscordDiffers(t *testing.T) {
	if DefaultDiscord() == legacyDiscord {
		t.Fatal("the old defaults must differ from the new ones")
	}
	if legacyDiscord == (Discord{}) {
		t.Fatal("the old defaults are empty")
	}
}
