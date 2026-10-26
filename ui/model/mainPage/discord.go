package mainpage

import (
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/discordrpc"
	"github.com/phantom-lancer/yatui/log"
)

const (
	// discordClientID is the application whose assets and name Discord shows.
	// These are the PulseSync values, so the card looks exactly like theirs.
	discordClientID = "1124055337234858005"
	// discordWebLink is the fallback second button of the card.
	discordWebLink = "https://github.com/phantom-lancer/yatui"
)

// discordTickMsg refreshes the presence on a timer, so a paused or finished
// track cannot stay "playing" in Discord forever.
type discordTickMsg struct{}

func discordTickCmd() tea.Cmd {
	return tea.Tick(10*time.Second, func(time.Time) tea.Msg { return discordTickMsg{} })
}

// discordConfig returns the Discord settings, creating defaults on first use.
func discordConfig() *config.Discord {
	if config.Current.Discord == nil {
		config.Current.Discord = &config.Discord{
			Enabled:       true,
			StatusType:    0,
			Language:      "en",
			ShowAlbum:     true,
			ShowVersion:   true,
			ShowSmallIcon: true,
			ShowButtons:   false,
			AfkTimeout:    15,
		}
	}
	return config.Current.Discord
}

// discordUpdate sends the current track to Discord. A missing Discord client
// is not an error: the next track just tries to connect again.
func (m *Model) discordUpdate(track *api.Track, playing bool) {
	cfg := discordConfig()
	if !cfg.Enabled {
		m.discordClear()
		return
	}
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

	if playing && stillPlaying {
		m.discordPausedAt = time.Time{}
	} else if m.discordPausedAt.IsZero() {
		m.discordPausedAt = time.Now()
	}

	if err := m.discord.SetActivity(buildDiscordActivity(track, position, playing, stillPlaying, cfg)); err != nil {
		log.Print(log.LVL_WARNIGN, "discord rpc: %s", err)
		_ = m.discord.Close()
		m.discord = nil
		return
	}
	m.discordTrack = track
}

// discordSync refreshes the card from the player state on the timer.
func (m *Model) discordSync() {
	if !discordConfig().Enabled {
		m.discordClear()
		return
	}
	if m.discordTrack == nil || m.tracker == nil {
		return
	}
	if m.tracker.IsStoped() {
		m.discordClear()
		return
	}
	if !m.tracker.IsPlaying() && !m.discordPausedAt.IsZero() {
		timeout := time.Duration(discordConfig().AfkTimeout) * time.Minute
		if time.Since(m.discordPausedAt) >= timeout {
			m.discordClear()
			return
		}
	}
	m.discordUpdate(m.discordTrack, m.tracker.IsPlaying())
}

// discordRefresh re-sends the current track after play/pause.
func (m *Model) discordRefresh() {
	if m.discordTrack == nil {
		return
	}
	m.discordUpdate(m.discordTrack, m.tracker != nil && m.tracker.IsPlaying())
}

// discordClear removes the card, e.g. when playback stops.
func (m *Model) discordClear() {
	if m.discord == nil {
		return
	}
	_ = m.discord.Clear()
}

// buildDiscordActivity copies the PulseSync card field by field and applies
// the user settings.
func buildDiscordActivity(track *api.Track, position time.Duration, playing, stillPlaying bool, cfg *config.Discord) discordrpc.Activity {
	title := track.Title
	if cfg.ShowVersion && track.Version != "" {
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

	largeText := "yatui " + Version
	if cfg.ShowAlbum && album != "" {
		largeText = album
	}

	activity := discordrpc.Activity{
		Details:           clip(title, 128),
		State:             clip(artist, 128),
		DetailsURL:        webLink(trackPath),
		StateURL:          webLink(artistPath),
		Type:              2, // listening
		StatusDisplayType: cfg.StatusType,
		Instance:          false,
		Assets: &discordrpc.Assets{
			LargeImage: cover,
			LargeText:  clip(largeText, 128),
			LargeURL:   firstNonEmpty(webLink(albumPath), discordWebLink),
		},
	}

	if cfg.ShowSmallIcon {
		activity.Assets.SmallImage = "playing"
		activity.Assets.SmallText = discordPlayingText(cfg.Language, true)
	}

	if playing && stillPlaying {
		start := time.Now().Add(-position).UnixMilli()
		activity.Timestamps = &discordrpc.Timestamps{
			Start: start,
			End:   start + int64(track.DurationMs),
		}
	} else {
		if cfg.ShowSmallIcon {
			activity.Assets.SmallImage = "paused"
			activity.Assets.SmallText = discordPlayingText(cfg.Language, false)
		}
	}

	if cfg.ShowButtons && trackPath != "" {
		if cfg.MergeLinks {
			activity.Buttons = []discordrpc.Button{
				{Label: "Listen on Yandex Music", URL: webLink(trackPath)},
			}
		} else {
			activity.Buttons = []discordrpc.Button{
				{Label: "Listen in Yandex Music App", URL: "yandexmusic://" + trackPath},
				{Label: "Listen in Yandex Music Web", URL: webLink(trackPath)},
			}
		}
	}

	return activity
}

// discordPlayingText localizes the small status line.
func discordPlayingText(language string, playing bool) string {
	if language == "ru" {
		if playing {
			return "Играет на DESKTOP"
		}
		return "Пауза на DESKTOP"
	}
	if playing {
		return "Playing on DESKTOP"
	}
	return "Paused on DESKTOP"
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
