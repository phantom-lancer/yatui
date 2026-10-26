package mainpage

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/ui/style"
)

// _SETTINGS_BUTTON_MARGIN is the space between the button and the border of
// the sidebar.
const _SETTINGS_BUTTON_MARGIN = 1

// settingsButtonView renders the clickable block that sits in the bottom left
// corner of the sidebar: the quick track search and the settings. The
// playlists component draws it and reports its area back, so clicks can be
// mapped to the halves.
func (m *Model) settingsButtonView() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.footerButton("Поиск", "/", m.searchHover),
		m.footerButton("Настройки", "F9", m.settingsHover),
	)
}

// footerButton renders one line of the sidebar footer.
func (m *Model) footerButton(label, hint string, hover bool) string {
	text := style.SettingsButtonTextStyle.Render(label)
	key := style.SettingsButtonHintStyle.Render(hint)

	// the block spans the whole sidebar, with a small margin on both sides:
	// it looks like a part of the panel and not a block in the corner
	width := m.playlists.Width()
	if width <= 0 {
		width = style.SidePanelWidth
	}
	inner := width - 2*_SETTINGS_BUTTON_MARGIN - style.SettingsButtonStyle.GetHorizontalFrameSize()
	if inner < lipgloss.Width(text)+lipgloss.Width(key)+1 {
		inner = lipgloss.Width(text) + lipgloss.Width(key) + 1
	}

	filler := inner - lipgloss.Width(text) - lipgloss.Width(key)
	content := text + strings.Repeat(" ", filler) + key

	if hover {
		return style.SettingsButtonActiveStyle.Render(content)
	}
	return style.SettingsButtonStyle.Render(content)
}
