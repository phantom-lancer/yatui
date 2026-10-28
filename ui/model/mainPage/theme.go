package mainpage

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/ui/style"
)

// cycleTheme switches to the next known theme and applies it on the fly.
func (m *Model) cycleTheme() tea.Cmd {
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

	return m.applyTheme(next)
}

// applyTheme replaces the colors, pushes them into every component and saves
// the choice to config.yaml. When the animated theme is chosen, the returned
// command starts its color loop.
func (m *Model) applyTheme(name string) tea.Cmd {
	colors, ok := config.ThemeColors(name)
	if !ok {
		return nil
	}

	// the colors the user had before the rgb theme are kept aside, so the
	// hidden reset can put everything back exactly as it was
	if name == "rgb" && config.Current.DevColors == nil && config.Current.Style != nil {
		config.Current.DevColors = config.Current.Style.Colors
	}
	if name != "rgb" && config.Current.DevColors != nil {
		config.Current.DevColors = nil
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

	if name == "rgb" {
		return m.rgbStart()
	}
	m.rgbRunning = false
	return nil
}

// themeReset restores the colors exactly as they are written in config.yaml,
// ignoring the theme overlay. Developer escape hatch: it exists to compare
// the system load with and without the rgb animation, so it is not bound to
// any keybind and is only reachable through the config value below.
func (m *Model) themeReset() {
	if config.Current.Style == nil {
		return
	}
	// clearing the theme name makes the config colors the source of truth
	// again, exactly like "as before"
	config.Current.Style.Theme = ""
	if config.Current.DevColors != nil {
		config.Current.Style.Colors = config.Current.DevColors
		config.Current.DevColors = nil
	}
	m.rgbRunning = false
	style.Apply(config.Current.Style)

	m.tracker.RefreshStyle()
	m.tracklist.RefreshStyle()
	m.playlists.RefreshStyle()
	_ = config.Save()

	m.showToast("Тема: как в конфиге")
}
