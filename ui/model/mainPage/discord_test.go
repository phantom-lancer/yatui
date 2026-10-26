package mainpage

import (
	"testing"
	"time"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
)

func testDiscordConfig() *config.Discord {
	return &config.Discord{
		Enabled:       true,
		StatusType:    0,
		Language:      "en",
		ShowAlbum:     true,
		ShowVersion:   true,
		ShowSmallIcon: true,
	}
}

// TestDiscordActivityMatchesPulseSync checks the fields copied from PulseSync.
func TestDiscordActivityMatchesPulseSync(t *testing.T) {
	track := &api.Track{
		Id:         "123",
		Title:      "Song",
		Version:    "remix",
		DurationMs: 180000,
		CoverUri:   "avatars.yandex.net/cover/%%",
		Albums:     []api.Album{{Id: 42, Title: "Album"}},
		Artists:    []api.Artist{{Id: 7, Name: "Artist"}},
	}

	activity := buildDiscordActivity(track, 60*time.Second, true, true, testDiscordConfig())

	if activity.Type != 2 {
		t.Errorf("type = %d, want 2 (listening)", activity.Type)
	}
	if activity.Details != "Song (remix)" {
		t.Errorf("details = %q", activity.Details)
	}
	if activity.State != "Artist" {
		t.Errorf("state = %q", activity.State)
	}
	if activity.DetailsURL != "https://music.yandex.ru/album/42/track/123?utm_source=discord&utm_medium=rich_presence_click" {
		t.Errorf("details url = %q", activity.DetailsURL)
	}
	if activity.StateURL != "https://music.yandex.ru/artist/7?utm_source=discord&utm_medium=rich_presence_click" {
		t.Errorf("state url = %q", activity.StateURL)
	}
	if activity.Assets.LargeImage != "https://avatars.yandex.net/cover/400x400" {
		t.Errorf("cover = %q", activity.Assets.LargeImage)
	}
	if activity.Assets.SmallImage != "playing" {
		t.Errorf("small image = %q", activity.Assets.SmallImage)
	}
	if len(activity.Buttons) != 0 {
		t.Errorf("buttons must be off by default, got %+v", activity.Buttons)
	}
	if activity.Timestamps == nil || activity.Timestamps.End-activity.Timestamps.Start != 180000 {
		t.Errorf("timestamps = %+v", activity.Timestamps)
	}

	paused := buildDiscordActivity(track, 60*time.Second, false, false, testDiscordConfig())
	if paused.Timestamps != nil {
		t.Error("paused activity must not have timestamps")
	}
	if paused.Assets.SmallImage != "paused" {
		t.Errorf("paused icon = %q", paused.Assets.SmallImage)
	}

	withButtons := testDiscordConfig()
	withButtons.ShowButtons = true
	buttons := buildDiscordActivity(track, 0, true, true, withButtons)
	if len(buttons.Buttons) != 2 {
		t.Errorf("buttons = %+v", buttons.Buttons)
	}

	merged := testDiscordConfig()
	merged.ShowButtons = true
	merged.MergeLinks = true
	mergedButtons := buildDiscordActivity(track, 0, true, true, merged)
	if len(mergedButtons.Buttons) != 1 {
		t.Errorf("merged buttons = %+v", mergedButtons.Buttons)
	}

	noIcon := testDiscordConfig()
	noIcon.ShowSmallIcon = false
	if got := buildDiscordActivity(track, 0, true, true, noIcon); got.Assets.SmallImage != "" {
		t.Error("small icon must be hidden")
	}
}

// TestDiscordUpdateWithoutDiscord does not panic when Discord is not running.
func TestDiscordUpdateWithoutDiscord(t *testing.T) {
	previous := config.Current.Discord
	t.Cleanup(func() { config.Current.Discord = previous })
	config.Current.Discord = testDiscordConfig()

	m := &Model{}
	m.discordUpdate(&api.Track{Title: "Song", DurationMs: 1000}, true)
}
