package config

import (
	"regexp"
	"testing"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{3}([0-9a-fA-F]{3})?$`)

func TestThemePresetsAreComplete(t *testing.T) {
	names := ThemeNames()
	if len(names) == 0 {
		t.Fatal("no themes registered")
	}

	seen := make(map[string]bool, len(names))
	for _, theme := range themes {
		if seen[theme.Name] {
			t.Fatalf("duplicate theme %q", theme.Name)
		}
		seen[theme.Name] = true

		colors := theme.Colors
		values := map[string]string{
			"accent":             colors.Accent,
			"error":              colors.Error,
			"border":             colors.Border,
			"background":         colors.Background,
			"playlist-selection": colors.PlaylistSelection,
			"active-text":        colors.ActiveText,
			"normal-text":        colors.NormalText,
			"inactive-text":      colors.InactiveText,
			"track-title-text":   colors.TrackTitleText,
			"track-version-text": colors.TrackVersionText,
			"track-artist-text":  colors.TrackArtistText,
			"lyrics-previous":    colors.LyricsPrevious,
			"lyrics-current":     colors.LyricsCurrent,
			"lyrics-next":        colors.LyricsNext,
		}
		for field, value := range values {
			if !hexColor.MatchString(value) {
				t.Fatalf("theme %q: %s is not a hex color: %q", theme.Name, field, value)
			}
		}
	}
}

func TestThemeColors(t *testing.T) {
	colors, ok := ThemeColors("nord")
	if !ok {
		t.Fatal("nord theme is missing")
	}
	if colors.Accent != "#88C0D0" {
		t.Fatalf("unexpected nord accent: %s", colors.Accent)
	}

	if _, ok := ThemeColors("does-not-exist"); ok {
		t.Fatal("unknown theme must not be found")
	}
}
