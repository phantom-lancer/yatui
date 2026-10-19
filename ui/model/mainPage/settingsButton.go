package mainpage

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dece2183/yamusic-tui/ui/style"
)

// _SETTINGS_BUTTON_WIDTH is the inner width of the settings button. The whole
// block is this width plus the border and the padding.
const _SETTINGS_BUTTON_WIDTH = 18

// settingsButtonView renders the clickable block that sits in the bottom left
// corner of the sidebar. The playlists component draws it and reports its
// area back, so a click can be mapped to it.
func (m *Model) settingsButtonView() string {
	label := style.SettingsButtonTextStyle.Render("Настройки")
	hint := style.SettingsButtonHintStyle.Render("F9")

	filler := _SETTINGS_BUTTON_WIDTH - lipgloss.Width(label) - lipgloss.Width(hint)
	if filler < 1 {
		filler = 1
	}
	content := label + strings.Repeat(" ", filler) + hint

	if m.settingsHover {
		return style.SettingsButtonActiveStyle.Render(content)
	}
	return style.SettingsButtonStyle.Render(content)
}
