package mainpage

import (
	"strconv"
	"strings"
	"time"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/discordrpc"
	"github.com/phantom-lancer/yatui/log"
)

const (
	// discordClientID is the application whose assets and name Discord shows.
	// These are the PulseSync values, so the card looks exactly like theirs.
	discordClientID = "1124055337234858005"
	// discordWebLink is the second button of the card.
	discordWebLink = "https://github.com/phantom-lancer/yatui"
)

// discordUpdate sends the current track to Discord. A missing Discord client
// is not an error: the next track just tries to connect again.
func (m *Model) discordUpdate(track *api.Track, playing bool) {
	if track == nil {
		return
	}

	if m.discord == nil {
		client, err := discordrpc.Dial(discordClientID)
		if err != nil {
			return
		}
		m.discord = client
	}

	position := time.Duration(0)
	stillPlaying := true
	if m.tracker != nil {
		if !m.tracker.IsStoped() {
			position = m.tracker.Position()
		}
		stillPlaying = m.tracker.IsPlaying()
	}

	if err := m.discord.SetActivity(buildDiscordActivity(track, position, playing, stillPlaying)); err != nil {
		log.Print(log.LVL_WARNIGN, "discord rpc: %s", err)
		_ = m.discord.Close()
		m.discord = nil
		return
	}
	m.discordTrack = track
}

// discordRefresh re-sends the current track after play/pause.
func (m *Model) discordRefresh() {
	if m.discordTrack == nil {
		return
	}
	m.discordUpdate(m.discordTrack, m.tracker.IsPlaying())
}

// discordClear removes the card, e.g. when playback stops.
func (m *Model) discordClear() {
	if m.discord == nil {
		return
	}
	_ = m.discord.Clear()
}

// discordClose closes the pipe on exit.
func (m *Model) discordClose() {
	if m.discord == nil {
		return
	}
	_ = m.discord.Close()
	m.discord = nil
}

// buildDiscordActivity copies the PulseSync card field by field.
func buildDiscordActivity(track *api.Track, position time.Duration, playing, stillPlaying bool) discordrpc.Activity {
	title := track.Title
	if track.Version != "" {
		title += " (" + track.Version + ")"
	}

	artists := make([]string, 0, len(track.Artists))
	for _, artist := range track.Artists {
		artists = append(artists, artist.Name)
	}
	artist := strings.Join(artists, ", ")

	album := ""
	albumID := ""
	if len(track.Albums) > 0 {
		album = track.Albums[0].Title
		albumID = strconv.FormatUint(track.Albums[0].Id, 10)
		if albumID == "0" {
			albumID = ""
		}
	}

	cover := track.CoverUri
	if track.OgImage != "" {
		cover = track.OgImage
	}
	if cover != "" {
		cover = "https://" + strings.Replace(cover, "%%", "400x400", 1)
	}

	trackPath := ""
	if albumID != "" && track.Id != "" {
		trackPath = "album/" + albumID + "/track/" + track.Id
	}

	albumPath := ""
	if albumID != "" {
		albumPath = "album/" + albumID
	}

	artistPath := ""
	if len(track.Artists) > 0 && track.Artists[0].Id != 0 {
		artistPath = "artist/" + strconv.FormatUint(track.Artists[0].Id, 10)
	}

	activity := discordrpc.Activity{
		Details:           clip(title, 128),
		State:             clip(artist, 128),
		DetailsURL:        webLink(trackPath),
		StateURL:          webLink(artistPath),
		Type:              2, // listening
		StatusDisplayType: 0,
		Instance:          false,
		Assets: &discordrpc.Assets{
			LargeImage: cover,
			LargeText:  clip(firstNonEmpty(album, "yatui "+Version), 128),
			LargeURL:   firstNonEmpty(webLink(albumPath), discordWebLink),
			SmallImage: "playing",
			SmallText:  "Playing on DESKTOP",
		},
	}

	if playing && stillPlaying {
		start := time.Now().Add(-position).UnixMilli()
		activity.Timestamps = &discordrpc.Timestamps{
			Start: start,
			End:   start + int64(track.DurationMs),
		}
	} else {
		activity.Assets.SmallImage = "paused"
		activity.Assets.SmallText = "Paused on DESKTOP"
	}

	if trackPath != "" {
		activity.Buttons = []discordrpc.Button{
			{Label: "Listen in Yandex Music App", URL: "yandexmusic://" + trackPath},
			{Label: "Listen in Yandex Music Web", URL: webLink(trackPath)},
		}
	}

	return activity
}

// webLink is the same referral link PulseSync puts into the card.
func webLink(path string) string {
	if path == "" {
		return ""
	}
	return "https://music.yandex.ru/" + path + "?utm_source=discord&utm_medium=rich_presence_click"
}

// clip keeps the field inside the Discord limit.
func clip(s string, limit int) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	if len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
