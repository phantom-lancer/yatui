package mainpage

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestSettingsButtonIsPlacedOnTheSidebar checks that the button block that
// opens the settings is drawn exactly where its clickable area is.
func TestSettingsButtonIsPlacedOnTheSidebar(t *testing.T) {
	playlists := testPlaylists(t, []string{"my wave", "liked tracks", "albums"})
	m := &Model{width: 100, height: 24, playlists: playlists}
	playlists.SetFooter(m.settingsButtonView())

	playlistView := playlists.View()
	tracklistView := testTracklist(t, []string{"first track", "second track"}).View()
	playerView, regions := trackerView(8)
	midPanel := lipgloss.JoinVertical(lipgloss.Left, tracklistView, playerView)
	view := lipgloss.JoinHorizontal(lipgloss.Bottom, playlistView, midPanel)

	layout := buildMouseLayout(false, false, playlistView, tracklistView, playerView, midPanel, regions)
	button := placeSidebarFooter(layout.playlist, toScreenRect(playlists.FooterRect()))

	if button.IsEmpty() {
		t.Fatal("the settings button has no area")
	}

	y := findLine(t, view, "Настройки")
	if y < button.Y || y >= button.Y+button.Height {
		t.Fatalf("the button is drawn on row %d, but its area is %+v", y, button)
	}

	line := lineOf(t, view, y)
	if got := fromCells(line, button.X); !strings.Contains(got, "Настройки") {
		t.Errorf("the button is not at column %d: %q", button.X, line)
	}

	// the whole block reacts to the mouse
	for row := button.Y; row < button.Y+button.Height; row++ {
		if !button.Contains(button.X, row) {
			t.Errorf("the button is not clickable at %d,%d", button.X, row)
		}
	}
}

// TestSettingsButtonIsInsideTheSidebar makes sure the button never sticks
// out of the sidebar, where it would cover the track list.
func TestSettingsButtonIsInsideTheSidebar(t *testing.T) {
	playlists := testPlaylists(t, []string{"my wave", "liked tracks"})
	m := &Model{width: 100, height: 24, playlists: playlists}
	playlists.SetFooter(m.settingsButtonView())

	playlistView := playlists.View()
	playerView, regions := trackerView(8)
	layout := buildMouseLayout(false, false, playlistView, "", playerView, playerView, regions)
	button := placeSidebarFooter(layout.playlist, toScreenRect(playlists.FooterRect()))

	if button.IsEmpty() {
		t.Fatal("the settings button has no area")
	}

	sidebarWidth := lipgloss.Width(playlistView)
	if button.X+button.Width > sidebarWidth {
		t.Errorf("the button (%+v) sticks out of the sidebar (%d wide)", button, sidebarWidth)
	}
	if button.X < 0 || button.Y < 0 {
		t.Errorf("the button is out of the screen: %+v", button)
	}
}
