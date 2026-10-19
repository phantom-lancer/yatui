package mainpage

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/dece2183/yamusic-tui/api"
	"github.com/dece2183/yamusic-tui/config"
	"github.com/dece2183/yamusic-tui/ui/components/playlist"
	"github.com/dece2183/yamusic-tui/ui/components/tracker"
	"github.com/dece2183/yamusic-tui/ui/components/tracklist"
	"github.com/dece2183/yamusic-tui/ui/style"
)

func strip(s string) string {
	return ansi.Strip(s)
}

func lineOf(t *testing.T, view string, y int) string {
	t.Helper()
	lines := strings.Split(view, "\n")
	if y < 0 || y >= len(lines) {
		t.Fatalf("line %d is out of range (%d lines)", y, len(lines))
	}
	return strip(lines[y])
}

// fromCells returns the part of the line starting at the given column.
func fromCells(line string, cols int) string {
	col := 0
	for i, r := range strip(line) {
		if col >= cols {
			return line[i:]
		}
		col += runewidth.RuneWidth(r)
	}
	return ""
}

// findLine returns the number of the line containing the substring.
func findLine(t *testing.T, view, sub string) int {
	t.Helper()
	for i, line := range strings.Split(view, "\n") {
		if strings.Contains(strip(line), sub) {
			return i
		}
	}
	t.Fatalf("substring %q is not found in the view", sub)
	return -1
}

func testPlaylists(t *testing.T, names []string) *playlist.Model {
	t.Helper()

	m := playlist.New(nil, "YaMusic")
	items := make([]*playlist.Item, len(names))
	for i, name := range names {
		items[i] = &playlist.Item{Name: name, Kind: playlist.USER, Active: true}
	}
	m.SetSize(style.SidePanelWidth, 24)
	m.SetItems(items)
	return m
}

func testTracklist(t *testing.T, titles []string) *tracklist.Model {
	t.Helper()

	likes := map[string]bool{}
	dislikes := map[string]bool{}
	cached := map[string]bool{}
	m := tracklist.New(nil, &likes, &dislikes, &cached)
	items := make([]tracklist.Item, len(titles))
	for i, title := range titles {
		items[i] = tracklist.Item{
			Track: &api.Track{
				Id:         fmt.Sprint(i + 1),
				Title:      title,
				DurationMs: 100000,
				Available:  true,
			},
			Artists: "artist",
		}
	}
	m.SetSize(60, 20)
	m.SetItems(items)
	return m
}

// trackerView builds a stand alone player view with recognizable lines.
func trackerView(height int) (string, *tracker.Regions) {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = fmt.Sprintf("TRACKER-LINE-%d", i)
	}

	regions := &tracker.Regions{
		PlayButton: tracker.Rect{X: 1, Y: 2, Width: 3, Height: 1},
		Progress:   tracker.Rect{X: 4, Y: 2, Width: 10, Height: 1},
		Volume:     tracker.Rect{X: 14, Y: 2, Width: 5, Height: 1},
		VolumeBar:  tracker.Rect{X: 16, Y: 2, Width: 3, Height: 1},
		Like:       tracker.Rect{X: 20, Y: 3, Width: 2, Height: 1},
	}

	return strings.Join(lines, "\n"), regions
}

func TestMouseLayoutMatchesComposedView(t *testing.T) {
	playlists := testPlaylists(t, []string{"my wave", "liked tracks", "albums"})
	tracks := testTracklist(t, []string{"first track", "second track", "third track"})

	playlistView := playlists.View()
	tracklistView := tracks.View()
	playerView, regions := trackerView(8)
	midPanel := lipgloss.JoinVertical(lipgloss.Left, tracklistView, playerView)
	view := lipgloss.JoinHorizontal(lipgloss.Bottom, playlistView, midPanel)

	layout := buildMouseLayout(false, false, playlistView, tracklistView, playerView, midPanel, regions)

	// playlist view is placed at the left edge
	if layout.playlist.X != 0 {
		t.Errorf("playlist x = %d", layout.playlist.X)
	}

	// middle panel starts right after the playlist
	playlistWidth := lipgloss.Width(playlistView)
	if layout.tracklist.X != playlistWidth {
		t.Errorf("tracklist x = %d, want %d", layout.tracklist.X, playlistWidth)
	}

	// player is the bottom part of the middle panel
	wantPlayerY := findLine(t, view, "TRACKER-LINE-0")
	if got := layout.play.Y - regions.PlayButton.Y; got != wantPlayerY {
		t.Errorf("player row = %d, want %d", got, wantPlayerY)
	}
	if got := layout.like.Y - regions.Like.Y; got != wantPlayerY {
		t.Errorf("like row = %d, want %d", got, wantPlayerY)
	}

	// tracklist is the top part of the middle panel:
	// every item must be found by its position in the composed view
	for i, title := range []string{"first track", "second track", "third track"} {
		y := findLine(t, view, title)
		if got := tracks.ItemAt(10, y-layout.tracklist.Y); got != i {
			t.Errorf("track %q: ItemAt = %d, want %d", title, got, i)
		}
	}

	// clicks land on the same characters in the composed view
	viewLine := lineOf(t, view, layout.play.Y)
	playerLine := lineOf(t, playerView, regions.PlayButton.Y)
	same := func(got, want string) bool {
		return strings.TrimRight(got, " ") == strings.TrimRight(want, " ")
	}
	if got, want := fromCells(viewLine, layout.play.X), fromCells(playerLine, regions.PlayButton.X); !same(got, want) {
		t.Errorf("play button mismatch: %q vs %q", got, want)
	}
	if got, want := fromCells(viewLine, layout.progress.X), fromCells(playerLine, regions.Progress.X); !same(got, want) {
		t.Errorf("progress bar mismatch: %q vs %q", got, want)
	}
	if got, want := fromCells(viewLine, layout.volume.X), fromCells(playerLine, regions.Volume.X); !same(got, want) {
		t.Errorf("volume indicator mismatch: %q vs %q", got, want)
	}
	if got, want := fromCells(viewLine, layout.volumeBar.X), fromCells(playerLine, regions.VolumeBar.X); !same(got, want) {
		t.Errorf("volume bar mismatch: %q vs %q", got, want)
	}
	if got := fromCells(lineOf(t, view, layout.like.Y), layout.like.X); got == "" {
		t.Errorf("like indicator is out of the view")
	}

	// a point inside the player must not address any track
	if got := tracks.ItemAt(layout.tracklist.X+10, layout.play.Y); got != -1 {
		t.Errorf("point inside the player must not hit a track, got %d", got)
	}
	// a point inside the playlist addresses its items
	for i, name := range []string{"my wave", "liked tracks", "albums"} {
		y := findLine(t, view, name)
		if got := playlists.ItemAt(10, y-layout.playlist.Y); got != i {
			t.Errorf("playlist %q: ItemAt = %d, want %d", name, got, i)
		}
	}
}

func TestMouseLayoutHiddenPanels(t *testing.T) {
	playlists := testPlaylists(t, []string{"my wave", "liked tracks"})
	playlistView := playlists.View()
	playerView, regions := trackerView(6)

	// only the player is visible
	layout := buildMouseLayout(true, false, playlistView, "", playerView, playerView, regions)
	if !layout.tracklist.IsEmpty() {
		t.Errorf("hidden tracklist must not have a region")
	}
	if layout.play.IsEmpty() {
		t.Errorf("player region is empty")
	}

	// only the tracklist is visible
	tracks := testTracklist(t, []string{"first track"})
	tracklistView := tracks.View()
	t.Logf("tracklist view: %dx%d", lipgloss.Width(tracklistView), lipgloss.Height(tracklistView))
	layout = buildMouseLayout(false, true, playlistView, tracklistView, "", tracklistView, nil)
	if layout.playlist.IsEmpty() {
		t.Errorf("playlist region is empty")
	}
	if layout.tracklist.IsEmpty() {
		t.Errorf("tracklist region is missing")
	}
	if !layout.play.IsEmpty() {
		t.Errorf("hidden player must not have a region")
	}
}

func init() {
	if err := config.InitialLoad(); err != nil {
		panic(err)
	}
	style.Apply(config.Current.Style)
}

// TestPlayerHotkeys checks that the function keys are recognized as global
// player hotkeys with the keys bubbletea reports for them.
func TestPlayerHotkeys(t *testing.T) {
	controls := config.Current.Controls

	for _, key := range []tea.KeyType{tea.KeyF6, tea.KeyF7, tea.KeyF8} {
		name := tea.KeyMsg{Type: key}.String()
		if !isPlayerHotkey(controls, name) {
			t.Errorf("%s (%q) is not a player hotkey", key, name)
		}
	}

	for _, keypress := range []string{"ctrl+q", "ctrl+c", "up", "down", "left", "right", "enter", "esc", " ", "?", "l", "L"} {
		if isPlayerHotkey(controls, keypress) {
			t.Errorf("%q must not be a player hotkey", keypress)
		}
	}
}

// TestPlayerHotkeyRouting checks the order of the key handling:
// player hotkeys are handled before any focused element.
func TestPlayerHotkeyRouting(t *testing.T) {
	controls := config.Current.Controls

	// simulate the order of the key switch of the main page
	for _, keypress := range []string{"f6", "f7", "f8"} {
		if !controls.Quit.Contains(keypress) && isPlayerHotkey(controls, keypress) {
			continue
		}
		t.Errorf("%q is handled after the focused element", keypress)
	}
}
