package mainpage

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dece2183/yamusic-tui/ui/style"
)

// _SETTINGS_BUTTON_MARGIN is the space between the button and the border of
// the sidebar.
const _SETTINGS_BUTTON_MARGIN = 1

// settingsButtonView renders the clickable block that sits in the bottom left
// corner of the sidebar. The playlists component draws it and reports its
// area back, so a click can be mapped to it.
func (m *Model) settingsButtonView() string {
	label := style.SettingsButtonTextStyle.Render("Настройки")
	hint := style.SettingsButtonHintStyle.Render("F9")

	// the block spans the whole sidebar, with a small margin on both sides:
	// it looks like a part of the panel and not a block in the corner
	width := m.playlists.Width()
	if width <= 0 {
		width = style.SidePanelWidth
	}
	inner := width - 2*_SETTINGS_BUTTON_MARGIN - style.SettingsButtonStyle.GetHorizontalFrameSize()
	if inner < lipgloss.Width(label)+lipgloss.Width(hint)+1 {
		inner = lipgloss.Width(label) + lipgloss.Width(hint) + 1
	}

	filler := inner - lipgloss.Width(label) - lipgloss.Width(hint)
	content := label + strings.Repeat(" ", filler) + hint

	if m.settingsHover {
		return style.SettingsButtonActiveStyle.Render(content)
	}
	return style.SettingsButtonStyle.Render(content)
}
