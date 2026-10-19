package config

// Theme is a named set of colors. Themes are applied on top of the colors
// from the config file: a theme always wins, and switching a theme at runtime
// simply replaces Style.Colors.
type Theme struct {
	Name   string
	Colors Colors
}

// ThemeDefaultName is used when no theme is configured: it keeps the colors
// exactly as the user wrote them in config.yaml.
const ThemeDefaultName = "yandex"

var themes = []Theme{
	{
		Name: "yandex",
		Colors: Colors{
			Accent:            "#FC0",
			Error:             "#F33",
			Border:            "#444",
			Background:        "#6b6b6b",
			PlaylistSelection: "#4a3c00",
			ActiveText:        "#EEE",
			NormalText:        "#CCC",
			InactiveText:      "#888",
			TrackTitleText:    "#dcdcdc",
			TrackVersionText:  "#999",
			TrackArtistText:   "#bbb",
			LyricsPrevious:    "#444",
			LyricsCurrent:     "#EEE",
			LyricsNext:        "#777",
		},
	},
	{
		Name: "nord",
		Colors: Colors{
			Accent:            "#88C0D0",
			Error:             "#BF616A",
			Border:            "#4C566A",
			Background:        "#434C5E",
			PlaylistSelection: "#3B4252",
			ActiveText:        "#ECEFF4",
			NormalText:        "#D8DEE9",
			InactiveText:      "#616E88",
			TrackTitleText:    "#E5E9F0",
			TrackVersionText:  "#81A1C1",
			TrackArtistText:   "#D8DEE9",
			LyricsPrevious:    "#4C566A",
			LyricsCurrent:     "#ECEFF4",
			LyricsNext:        "#81A1C1",
		},
	},
	{
		Name: "gruvbox",
		Colors: Colors{
			Accent:            "#FABD2F",
			Error:             "#FB4934",
			Border:            "#504945",
			Background:        "#3C3836",
			PlaylistSelection: "#504945",
			ActiveText:        "#FBF1C7",
			NormalText:        "#EBDBB2",
			InactiveText:      "#928374",
			TrackTitleText:    "#FBF1C7",
			TrackVersionText:  "#A89984",
			TrackArtistText:   "#D5C4A1",
			LyricsPrevious:    "#665C54",
			LyricsCurrent:     "#FBF1C7",
			LyricsNext:        "#A89984",
		},
	},
	{
		Name: "dracula",
		Colors: Colors{
			Accent:            "#BD93F9",
			Error:             "#FF5555",
			Border:            "#44475A",
			Background:        "#44475A",
			PlaylistSelection: "#44475A",
			ActiveText:        "#F8F8F2",
			NormalText:        "#F8F8F2",
			InactiveText:      "#6272A4",
			TrackTitleText:    "#F8F8F2",
			TrackVersionText:  "#6272A4",
			TrackArtistText:   "#C3C7E6",
			LyricsPrevious:    "#44475A",
			LyricsCurrent:     "#F8F8F2",
			LyricsNext:        "#6272A4",
		},
	},
	{
		Name: "catppuccin",
		Colors: Colors{
			Accent:            "#CBA6F7",
			Error:             "#F38BA8",
			Border:            "#45475A",
			Background:        "#313244",
			PlaylistSelection: "#45475A",
			ActiveText:        "#CDD6F4",
			NormalText:        "#BAC2DE",
			InactiveText:      "#6C7086",
			TrackTitleText:    "#CDD6F4",
			TrackVersionText:  "#6C7086",
			TrackArtistText:   "#A6ADC8",
			LyricsPrevious:    "#45475A",
			LyricsCurrent:     "#CDD6F4",
			LyricsNext:        "#6C7086",
		},
	},
	{
		Name: "mono",
		Colors: Colors{
			Accent:            "#FFFFFF",
			Error:             "#FF5555",
			Border:            "#404040",
			Background:        "#303030",
			PlaylistSelection: "#3A3A3A",
			ActiveText:        "#FFFFFF",
			NormalText:        "#C0C0C0",
			InactiveText:      "#707070",
			TrackTitleText:    "#E8E8E8",
			TrackVersionText:  "#808080",
			TrackArtistText:   "#A0A0A0",
			LyricsPrevious:    "#404040",
			LyricsCurrent:     "#FFFFFF",
			LyricsNext:        "#808080",
		},
	},
}

// ThemeNames returns every known theme in switching order.
func ThemeNames() []string {
	names := make([]string, len(themes))
	for i, t := range themes {
		names[i] = t.Name
	}
	return names
}

// ThemeColors returns the colors of a theme by its name.
func ThemeColors(name string) (Colors, bool) {
	for _, t := range themes {
		if t.Name == name {
			return t.Colors, true
		}
	}
	return Colors{}, false
}
