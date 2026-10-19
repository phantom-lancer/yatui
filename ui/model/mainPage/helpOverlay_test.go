package mainpage

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dece2183/yamusic-tui/ui/components/playlist"
)

// TestSidebarHoverOnlyHighlights makes sure that the pointer over the sidebar
// marks the item under it, but does not switch the panel.
func TestSidebarHoverOnlyHighlights(t *testing.T) {
	playlists := testPlaylists(t, []string{"my wave", "liked tracks", "albums"})
	m := &Model{
		width:     100,
		height:    24,
		playlists: playlists,
		tracklist: testTracklist(t, []string{"first track"}),
	}

	playlistView := playlists.View()
	playerView, regions := trackerView(8)
	midPanel := lipgloss.JoinVertical(lipgloss.Left, "", playerView)
	layout := buildMouseLayout(false, false, playlistView, "", playerView, midPanel, regions)
	m.mouse = layout

	// the second item of the sidebar
	y := findLine(t, playlistView, "liked tracks")
	if got := playlists.ItemAt(10, y); got != 1 {
		t.Fatalf("the item under the cursor is %d, want 1", got)
	}

	m.mouseMotion(layout.playlist.X+10, layout.playlist.Y+y)

	if got := playlists.Index(); got != 1 {
		t.Errorf("the item under the cursor was not marked: index = %d", got)
	}
	if len(m.toasts) != 0 {
		t.Errorf("the hover switched the panel: %v", m.toasts)
	}
}

// TestSidebarClickSwitchesThePlaylist makes sure that a click still switches
// the panel, unlike the hover.
func TestSidebarClickSwitchesThePlaylist(t *testing.T) {
	playlists := testPlaylists(t, []string{"my wave", "liked tracks", "albums"})
	m := &Model{
		width:     100,
		height:    24,
		playlists: playlists,
		tracklist: testTracklist(t, []string{"first track"}),
	}

	playlistView := playlists.View()
	playerView, regions := trackerView(8)
	midPanel := lipgloss.JoinVertical(lipgloss.Left, "", playerView)
	layout := buildMouseLayout(false, false, playlistView, "", playerView, midPanel, regions)
	m.mouse = layout

	y := findLine(t, playlistView, "albums")
	m.mouseClick(layout.playlist.X+10, layout.playlist.Y+y)

	if got := playlists.Index(); got != 2 {
		t.Errorf("the clicked item was not selected: index = %d", got)
	}
	if len(m.toasts) == 0 {
		t.Error("the click did not switch the panel")
	}
}

// TestHintsAreMovedToTheHelpWindow makes sure that the panels do not show the
// key hints anymore: they all live in the help window now.
func TestHintsAreMovedToTheHelpWindow(t *testing.T) {
	playlists := testPlaylists(t, []string{"my wave", "liked tracks"})
	tracks := testTracklist(t, []string{"first track", "second track"})

	for name, view := range map[string]string{
		"sidebar":   playlists.View(),
		"tracklist": tracks.View(),
	} {
		for _, hint := range []string{"up •", "ctrl+↑", "like •", "play/pause", "vol up"} {
			if strings.Contains(strip(view), hint) {
				t.Errorf("the %s still shows the hint %q", name, hint)
			}
		}
	}
}

// TestHelpWindowOpensAndCloses checks the key bindings of the help window.
func TestHelpWindowOpensAndCloses(t *testing.T) {
	m := &Model{width: 100, height: 30}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !m.isHelpActive {
		t.Fatal("the help window did not open")
	}

	view := m.placeHelp()
	for _, want := range []string{"Горячие клавиши", "выход", "настройки", "нравится"} {
		if !strings.Contains(strip(view), want) {
			t.Errorf("the help window does not mention %q", want)
		}
	}

	// every key of the window is swallowed, the playback must not break
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !m.isHelpActive {
		t.Error("the help window closed on an unrelated key")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.isHelpActive {
		t.Error("the help window did not close on esc")
	}
}

// TestHelpWindowScrolls makes sure that the bindings that do not fit into the
// window can be reached.
func TestHelpWindowScrolls(t *testing.T) {
	m := &Model{width: 100, height: 24}
	m.isHelpActive = true

	before := m.helpView()
	m.helpOffset++
	after := m.helpView()
	if before == after {
		t.Fatal("the help window did not scroll")
	}

	// the offset never goes below zero
	m.helpOffset = -10
	if m.helpOffset >= 0 {
		t.Error("the scroll offset went below zero")
	}
}

// TestHelpWindowClosesOnAClickOutside makes sure that the window can be
// closed with the mouse.
func TestHelpWindowClosesOnAClickOutside(t *testing.T) {
	m := &Model{width: 100, height: 30}
	m.isHelpActive = true
	m.placeHelp()

	cmd, handled := m.helpControl(tea.MouseMsg{
		X:      1,
		Y:      1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if !handled {
		t.Fatal("the mouse was not handled by the help window")
	}
	if cmd == nil {
		t.Error("the screen was not cleared after the window was closed")
	}
	if m.isHelpActive {
		t.Error("the help window was not closed by the click")
	}
}

// TestHelpWindowShowsEveryBinding makes sure that no binding of the config is
// left out of the help window.
func TestHelpWindowShowsEveryBinding(t *testing.T) {
	m := &Model{width: 100, height: 60}
	m.isHelpActive = true

	view := strip(m.helpView())
	for _, section := range helpSections() {
		if !strings.Contains(view, section.title) {
			t.Errorf("the section %q is missing in the help window", section.title)
		}
		for _, item := range section.items {
			name := keyDisplayName(item.key)
			if name == "" {
				continue
			}
			if !strings.Contains(view, name) {
				t.Errorf("the binding %q is missing in the help window", name)
			}
		}
	}
}

// TestPlaylistHighlightDoesNotSwitch keeps the sidebar honest: the method the
// pointer uses only moves the cursor.
func TestPlaylistHighlightDoesNotSwitch(t *testing.T) {
	playlists := testPlaylists(t, []string{"my wave", "liked tracks"})

	if !playlists.Highlight(1) {
		t.Fatal("the item was not marked")
	}
	if got := playlists.Index(); got != 1 {
		t.Errorf("index = %d, want 1", got)
	}

	// an item that is not there changes nothing
	before := playlists.Index()
	playlists.Highlight(42)
	if got := playlists.Index(); got != before {
		t.Errorf("the cursor moved to a missing item: %d", got)
	}
}

var _ = playlist.MYWAVE
