package mainpage

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/ui/components/search"
)

// TestSearchDialogMousePicksASuggestion makes sure that a suggestion can be
// chosen with the mouse.
func TestSearchDialogMousePicksASuggestion(t *testing.T) {
	m := &Model{width: 100, height: 30}
	m.searchDialog = search.New()
	m.searchDialog.SetSize(56, 20)
	m.searchDialog.SetSuggestions([]string{"first playlist", "second playlist"})
	m.isAddPlaylistActive = true

	view := m.searchDialog.View()
	x := (m.width - lipgloss.Width(view)) / 2
	y := (m.height - lipgloss.Height(view)) / 2

	// the second suggestion is found under its name
	row := -1
	for i, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "second playlist") {
			row = i
			break
		}
	}
	if row < 0 {
		t.Fatal("the suggestion is not in the view")
	}

	index := m.searchDialog.ItemAt(2, row)
	if index != 1 {
		t.Fatalf("the suggestion under the cursor is %d, want 1", index)
	}

	// the mouse picks it
	m.searchMouse(tea.MouseMsg{
		X:      x + 2,
		Y:      y + row,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})

	value, ok := m.searchDialog.SuggestionValue()
	if !ok || value != "second playlist" {
		t.Fatalf("the picked suggestion is %q", value)
	}
}

// TestSearchDialogMouseHovers makes sure that the pointer marks the
// suggestion it is over.
func TestSearchDialogMouseHovers(t *testing.T) {
	m := &Model{width: 100, height: 30}
	m.searchDialog = search.New()
	m.searchDialog.SetSize(56, 20)
	m.searchDialog.SetSuggestions([]string{"first playlist", "second playlist"})
	m.isAddPlaylistActive = true

	view := m.searchDialog.View()
	x := (m.width - lipgloss.Width(view)) / 2
	y := (m.height - lipgloss.Height(view)) / 2

	row := -1
	for i, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "second playlist") {
			row = i
			break
		}
	}
	if row < 0 {
		t.Fatal("the suggestion is not in the view")
	}

	m.searchMouse(tea.MouseMsg{
		X:      x + 2,
		Y:      y + row,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonNone,
	})

	// the pointer does not pick the suggestion, it only marks it
	if value, ok := m.searchDialog.SuggestionValue(); ok {
		t.Fatalf("the suggestion was picked by the pointer: %q", value)
	}
}
