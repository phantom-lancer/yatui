package mainpage

import (
	"github.com/dece2183/yamusic-tui/config"
	"github.com/dece2183/yamusic-tui/log"
	"github.com/dece2183/yamusic-tui/ui/style"
)

// cycleTheme switches to the next known theme and applies it on the fly.
func (m *Model) cycleTheme() {
	names := config.ThemeNames()
	next := names[0]

	current := ""
	if config.Current.Style != nil {
		current = config.Current.Style.Theme
	}

	for i, name := range names {
		if name == current {
			next = names[(i+1)%len(names)]
			break
		}
	}

	m.applyTheme(next)
}

// applyTheme replaces the colors, pushes them into every component and saves
// the choice to config.yaml.
func (m *Model) applyTheme(name string) {
	colors, ok := config.ThemeColors(name)
	if !ok {
		return
	}

	config.Current.Style.Theme = name
	config.Current.Style.Colors = &colors
	style.Apply(config.Current.Style)

	m.tracker.RefreshStyle()
	m.tracklist.RefreshStyle()
	m.playlists.RefreshStyle()

	if err := config.Save(); err != nil {
		log.Print(log.LVL_WARNIGN, "failed to save config: %s", err)
	}

	m.showToast("Тема: " + name)
}
