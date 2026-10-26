package mainpage

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/phantom-lancer/yatui/ui/components/tracker"
)

// TestEqualizerToggleAlignment guards the byte-vs-cell index bug: the toggle
// must be measured in terminal cells, not in UTF-8 bytes, otherwise Cyrillic
// shifts its hitbox to the right.
func TestEqualizerToggleAlignment(t *testing.T) {
	t.Setenv("USERPROFILE", t.TempDir())

	m := &Model{width: 120, height: 40}
	m.tracker = &tracker.Model{}
	m.eq = &eqModel{enabled: true, preset: "Плоский", index: -1}
	placed := m.placeEqualizer()
	l := m.eq.layout

	lines := strings.Split(placed, "\n")
	if l.toggleRect.Y < 0 || l.toggleRect.Y >= len(lines) {
		t.Fatalf("toggle row %d is outside the view", l.toggleRect.Y)
	}
	row := ansi.Strip(lines[l.toggleRect.Y])
	cell := string([]rune(row)[l.toggleRect.X:])
	if !strings.HasPrefix(cell, "вкл") {
		t.Fatalf("toggle hitbox starts at %d, but the row there is %q", l.toggleRect.X, cell)
	}

	// the whole switch reacts, including its right edge
	for dx := 0; dx < l.toggleRect.Width; dx++ {
		before := m.eq.enabled
		m.eqMouse(tea.MouseMsg{
			X:      l.toggleRect.X + dx,
			Y:      l.toggleRect.Y,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionPress,
		})
		if m.eq.enabled == before {
			t.Errorf("the toggle does not react at dx=%d", dx)
		}
		m.eq.enabled = before
	}
}
