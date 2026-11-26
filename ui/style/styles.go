package style

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/phantom-lancer/yatui/config"
)

var (
	VolumeIndicatorWidth    = 16
	VolumeIndicatorAutohide = 64
	SidePanelWidth          = 32
	SidePanelAutohide       = 64
	SearchModalWidth        = 56
)

var (
	AccentColor            lipgloss.Color
	ErrorColor             lipgloss.Color
	BorderColor            lipgloss.Color
	BackgroundColor        lipgloss.Color
	PlaylistSelectionColor lipgloss.Color
	ActiveTextColor        lipgloss.Color
	NormalTextColor        lipgloss.Color
	InactiveTextColor      lipgloss.Color
	TrackTitleTextColor    lipgloss.Color
	TrackVersionTextColor  lipgloss.Color
	TrackArtistTextColor   lipgloss.Color
	LyricsPreviosTextColor lipgloss.Color
	LyricsCurrentTextColor lipgloss.Color
	LyricsNextTextColor    lipgloss.Color
)

var (
	IconPlay       = "▶"
	IconStop       = "■"
	IconLiked      = "💛"
	IconNotLiked   = "🤍"
	IconDisliked   = "👎"
	IconPrev       = "⏮"
	IconNext       = "⏭"
	IconCached     = "💿"
	IconDotLight   = "•"
	IconDotDark    = "•"
	IconVolumeOff  = "🔇"
	IconVolumeLow  = "🔈"
	IconVolumeMid  = "🔉"
	IconVolumeHigh = "🔊"
)

var (
	AccentTextStyle lipgloss.Style
	ErrorTextStyle  lipgloss.Style
)

var (
	DialogTitleStyle lipgloss.Style
	DialogBoxStyle   lipgloss.Style
	DialogHelpStyle  lipgloss.Style
)

var (
	ButtonStyle       lipgloss.Style
	ActiveButtonStyle lipgloss.Style
)

var (
	SideBoxStyle             lipgloss.Style
	SideBoxItemStyle         lipgloss.Style
	SideBoxSelItemStyle      lipgloss.Style
	SideBoxInactiveItemStyle lipgloss.Style
	// SideBoxHoverItemStyle marks the item under the mouse pointer.
	SideBoxHoverItemStyle  lipgloss.Style
	SideBoxSubItemStyle    lipgloss.Style
	SideBoxSelSubItemStyle lipgloss.Style
)

var (
	TrackBoxStyle      lipgloss.Style
	TrackTitleStyle    lipgloss.Style
	TrackVersionStyle  lipgloss.Style
	TrackArtistStyle   lipgloss.Style
	TrackProgressStyle lipgloss.Style
	TrackAddInfoStyle  lipgloss.Style
)

var (
	TrackListTitleStyle  lipgloss.Style
	TrackListStyle       lipgloss.Style
	TrackListActiveStyle lipgloss.Style
)

var (
	ToastStyle       lipgloss.Style
	ToastAccentStyle lipgloss.Style
)

// The shared building blocks of the interface. Keeping them here means the
// hierarchy (a section label, a muted hint, an accented badge) looks the same
// in every window instead of being reinvented by each component.
var (
	// SectionLabelStyle marks a group of rows: small, accented, bold.
	SectionLabelStyle lipgloss.Style
	// MutedStyle is the secondary text: counts, hints, timestamps.
	MutedStyle lipgloss.Style
	// BadgeStyle is a compact state marker, e.g. "офлайн" or "кэш".
	BadgeStyle lipgloss.Style
	// SidebarTitleStyle is the title of the playlist panel.
	SidebarTitleStyle lipgloss.Style
	// TrackListCountStyle is the muted count next to the track list title.
	TrackListCountStyle lipgloss.Style
	// KeyHintStyle is the key cap of a footer button or a help line.
	KeyHintStyle lipgloss.Style
	// BorderStyle paints the frame of a panel without re-measuring it.
	BorderStyle lipgloss.Style
)

// SettingsButton is the small clickable block in the corner of the sidebar.
var (
	SettingsButtonStyle       lipgloss.Style
	SettingsButtonActiveStyle lipgloss.Style
	SettingsButtonTextStyle   lipgloss.Style
	SettingsButtonHintStyle   lipgloss.Style
)

func Apply(style *config.Style) {
	VolumeIndicatorWidth = style.VolumeIndicatorWidth
	VolumeIndicatorAutohide = style.VolumeIndicatorAutohide
	SidePanelWidth = style.SidePanelWidth
	SidePanelAutohide = style.SidePanelAutohide
	SearchModalWidth = style.SearchModalWidth

	AccentColor = lipgloss.Color(style.Colors.Accent)
	ErrorColor = lipgloss.Color(style.Colors.Error)
	BorderColor = lipgloss.Color(style.Colors.Border)
	BackgroundColor = lipgloss.Color(style.Colors.Background)
	PlaylistSelectionColor = lipgloss.Color(style.Colors.PlaylistSelection)
	ActiveTextColor = lipgloss.Color(style.Colors.ActiveText)
	NormalTextColor = lipgloss.Color(style.Colors.NormalText)
	InactiveTextColor = lipgloss.Color(style.Colors.InactiveText)
	TrackTitleTextColor = lipgloss.Color(style.Colors.TrackTitleText)
	TrackVersionTextColor = lipgloss.Color(style.Colors.TrackVersionText)
	TrackArtistTextColor = lipgloss.Color(style.Colors.TrackArtistText)
	LyricsPreviosTextColor = lipgloss.Color(style.Colors.LyricsPrevious)
	LyricsCurrentTextColor = lipgloss.Color(style.Colors.LyricsCurrent)
	LyricsNextTextColor = lipgloss.Color(style.Colors.LyricsNext)

	IconPlay = style.Icons.Play
	IconStop = style.Icons.Stop
	IconLiked = style.Icons.Liked
	IconNotLiked = style.Icons.NotLiked
	IconDisliked = style.Icons.Disliked
	IconPrev = style.Icons.Prev
	IconNext = style.Icons.Next
	IconCached = style.Icons.Cached
	IconDotLight = lipgloss.NewStyle().Foreground(LyricsCurrentTextColor).Render(style.Icons.LyricsDot)
	IconDotDark = lipgloss.NewStyle().Foreground(LyricsPreviosTextColor).Render(style.Icons.LyricsDot)
	IconVolumeOff = style.Icons.VolumeOff
	IconVolumeLow = style.Icons.VolumeLow
	IconVolumeMid = style.Icons.VolumeMid
	IconVolumeHigh = style.Icons.VolumeHigh

	AccentTextStyle = lipgloss.NewStyle().Foreground(AccentColor)
	ErrorTextStyle = lipgloss.NewStyle().Foreground(ErrorColor)

	DialogTitleStyle = lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true).
		MarginLeft(2).
		MarginBottom(1)
	DialogBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(AccentColor).
		Padding(1, 2).
		BorderTop(true).
		BorderLeft(true).
		BorderRight(true).
		BorderBottom(true)

	SectionLabelStyle = lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true)
	MutedStyle = lipgloss.NewStyle().
		Foreground(InactiveTextColor)
	BadgeStyle = lipgloss.NewStyle().
		Foreground(BackgroundColor).
		Background(AccentColor).
		Padding(0, 1)
	SidebarTitleStyle = lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true).
		Padding(0, 1)
	TrackListCountStyle = lipgloss.NewStyle().
		Foreground(InactiveTextColor)
	KeyHintStyle = lipgloss.NewStyle().
		Foreground(InactiveTextColor)
	BorderStyle = lipgloss.NewStyle().
		Foreground(BorderColor)
	DialogHelpStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		PaddingTop(1)

	ButtonStyle = lipgloss.NewStyle().
		Foreground(NormalTextColor).
		Background(InactiveTextColor).
		Padding(0, 3).
		MarginTop(1)
	ActiveButtonStyle = ButtonStyle.
		Foreground(InactiveTextColor).
		Background(AccentColor)

	SideBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Width(SidePanelWidth).
		Padding(1, 0)
	SideBoxItemStyle = lipgloss.NewStyle().
		Foreground(NormalTextColor).
		PaddingLeft(2).
		Width(SidePanelWidth).
		MaxWidth(SidePanelWidth)
	SideBoxSelItemStyle = SideBoxItemStyle.
		Foreground(ActiveTextColor).
		Background(PlaylistSelectionColor).
		PaddingLeft(1).
		Border(lipgloss.InnerHalfBlockBorder()).
		BorderForeground(AccentColor).
		BorderTop(false).
		BorderLeft(true).
		BorderRight(false).
		BorderBottom(false)
	SideBoxInactiveItemStyle = SideBoxItemStyle.
		Foreground(InactiveTextColor).
		Padding(0, 0, 0, 2)
	// SideBoxHoverItemStyle marks the item under the mouse pointer.
	SideBoxHoverItemStyle = SideBoxItemStyle.
		Foreground(AccentColor)
	SideBoxSubItemStyle = SideBoxItemStyle.
		Padding(0, 0, 0, 4)
	SideBoxSelSubItemStyle = SideBoxSelItemStyle.
		Padding(0, 0, 0, 3)

	TrackBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(1, 2)
	TrackTitleStyle = lipgloss.NewStyle().
		Foreground(TrackTitleTextColor).
		Bold(true)
	TrackVersionStyle = lipgloss.NewStyle().
		Foreground(TrackVersionTextColor)
	TrackArtistStyle = lipgloss.NewStyle().
		Foreground(TrackArtistTextColor)
	TrackProgressStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		PaddingBottom(1)
	TrackAddInfoStyle = lipgloss.NewStyle().
		Align(lipgloss.Right)

	TrackListTitleStyle = lipgloss.NewStyle().
		Foreground(NormalTextColor).
		UnsetBackground()
	TrackListStyle = lipgloss.NewStyle().
		Padding(0, 2, 1)
	TrackListActiveStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(AccentColor).
		Background(PlaylistSelectionColor)

	ToastStyle = lipgloss.NewStyle().
		Foreground(ActiveTextColor).
		Background(PlaylistSelectionColor).
		Bold(false).
		Padding(0, 1)
	ToastAccentStyle = lipgloss.NewStyle().
		Foreground(AccentColor).
		Background(PlaylistSelectionColor)

	SettingsButtonStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(0, 1)
	SettingsButtonActiveStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(AccentColor).
		Padding(0, 1)
	SettingsButtonTextStyle = lipgloss.NewStyle().
		Foreground(NormalTextColor)
	SettingsButtonHintStyle = lipgloss.NewStyle().
		Foreground(InactiveTextColor)
}
