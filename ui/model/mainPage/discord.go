package mainpage

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
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
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return discordTickMsg{} })
}

// The Discord pipe is owned by one background goroutine: dialing a stuck
// Discord client must never block the UI thread.
var (
	discordQueue    = make(chan func(), 64)
	discordQueueOne sync.Once
	discordConn     *discordrpc.Client
	discordLastKey  string
	discordLastSent time.Time
)

func discordAsync(fn func()) {
	discordQueueOne.Do(func() {
		go func() {
			for job := range discordQueue {
				job()
			}
		}()
	})
	select {
	case discordQueue <- fn:
	default:
	}
}

// discordSendAsync sends the card in the background, skipping repeats.
func discordSendAsync(activity discordrpc.Activity, key string) {
	discordAsync(func() {
		if discordConn == nil || discordConn.Dead() {
			if discordConn != nil {
				_ = discordConn.Close()
				discordConn = nil
			}
			client, err := discordrpc.Dial(discordClientID)
			if err != nil {
				return
			}
			discordConn = client
			discordLastKey = ""
		}
		if key == discordLastKey && time.Since(discordLastSent) < time.Minute {
			return
		}
		if err := discordConn.SetActivity(activity); err != nil {
			log.Print(log.LVL_WARNIGN, "discord rpc: %s", err)
			_ = discordConn.Close()
			discordConn = nil
			discordLastKey = ""
			return
		}
		discordLastKey = key
		discordLastSent = time.Now()
	})
}

// discordClearAsync removes the card in the background.
func discordClearAsync() {
	discordAsync(func() {
		if discordConn != nil {
			_ = discordConn.Clear()
			_ = discordConn.Close()
			discordConn = nil
		}
		discordLastKey = ""
		discordLastSent = time.Now()
	})
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
			ShowBranding:  true,
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

	position := time.Duration(0)
	if m.tracker != nil && !m.tracker.IsStoped() {
		position = m.tracker.Position()
	}

	if playing {
		m.discordPausedAt = time.Time{}
	} else if m.discordPausedAt.IsZero() {
		m.discordPausedAt = time.Now()
	}

	activity := buildDiscordActivity(track, position, playing, cfg)
	key := discordActivityKey(activity, playing)
	m.discordTrack = track
	discordSendAsync(activity, key)
}

// discordActivityKey serializes everything except the timestamps: Discord
// counts the time itself, so a new timestamp alone is not a change. The
// playing flag is included, otherwise pausing would look identical.
func discordActivityKey(activity discordrpc.Activity, playing bool) string {
	activity.Timestamps = nil
	payload, _ := json.Marshal(activity)
	if playing {
		return string(payload) + "|playing"
	}
	return string(payload) + "|paused"
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

// discordRefresh re-sends the current track after play/pause. The state comes
// from the key press, not from the player: at the moment the command arrives
// the player has not switched yet, so IsPlaying would report the old state.
func (m *Model) discordRefresh(playing bool) {
	if m.discordTrack == nil {
		return
	}
	m.discordUpdate(m.discordTrack, playing)
}

// discordClear removes the card, e.g. when playback stops.
func (m *Model) discordClear() {
	discordClearAsync()
}

// buildDiscordActivity copies the PulseSync card field by field and applies
// the user settings.
func buildDiscordActivity(track *api.Track, position time.Duration, playing bool, cfg *config.Discord) discordrpc.Activity {
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

	// PulseSync uses the cover of the track; OgImage is only a fallback and
	// may already contain the scheme, so it must not be prefixed twice.
	cover := track.CoverUri
	if cover == "" {
		cover = track.OgImage
	}
	if cover != "" {
		if !strings.HasPrefix(cover, "http") {
			cover = "https://" + cover
		}
		cover = strings.Replace(cover, "%%", "400x400", 1)
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

	largeText := ""
	if cfg.ShowBranding {
		largeText = "yatui " + Version
	}
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

	if playing {
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
