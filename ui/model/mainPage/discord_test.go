package mainpage

import (
	"testing"
	"time"

	"github.com/phantom-lancer/yatui/api"
)

// TestDiscordActivityMatchesPulseSync checks the fields we copy from PulseSync.
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

	activity := buildDiscordActivity(track, 60*time.Second, true, true)

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
	if len(activity.Buttons) != 2 || activity.Buttons[1].URL == "" {
		t.Errorf("buttons = %+v", activity.Buttons)
	}
	if activity.Timestamps == nil || activity.Timestamps.End-activity.Timestamps.Start != 180000 {
		t.Errorf("timestamps = %+v", activity.Timestamps)
	}

	paused := buildDiscordActivity(track, 60*time.Second, false, false)
	if paused.Timestamps != nil {
		t.Error("paused activity must not have timestamps")
	}
	if paused.Assets.SmallImage != "paused" {
		t.Errorf("paused icon = %q", paused.Assets.SmallImage)
	}
}

// TestDiscordUpdateWithoutDiscord does not panic when Discord is not running.
func TestDiscordUpdateWithoutDiscord(t *testing.T) {
	m := &Model{}
	m.discordUpdate(&api.Track{Title: "Song", DurationMs: 1000}, true)
}
