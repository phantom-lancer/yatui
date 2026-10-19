package playlist

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestFooterKeepsTheSidebarHeight(t *testing.T) {
	m := newTestModel(t, []string{"my wave", "liked tracks", "albums"}, 32, 20)
	without := lipgloss.Height(m.View())

	m.SetFooter("SETTINGS-BLOCK\nSECOND-LINE\nTHIRD-LINE")
	with := lipgloss.Height(m.View())

	if without != with {
		t.Fatalf("the footer changed the height of the sidebar: %d -> %d", without, with)
	}
}

func TestFooterIsClickable(t *testing.T) {
	m := newTestModel(t, []string{"my wave", "liked tracks", "albums"}, 32, 20)
	m.SetFooter("SETTINGS-BLOCK\nSECOND-LINE")

	view := m.View()
	rect := m.FooterRect()
	if rect.IsEmpty() {
		t.Fatal("the footer has no area")
	}

	lines := strings.Split(strip(view), "\n")
	if len(lines) < rect.Y+rect.Height {
		t.Fatalf("the footer sticks out of the view: %+v (%d lines)", rect, len(lines))
	}

	for i := 0; i < rect.Height; i++ {
		line := lines[rect.Y+i]
		if !strings.Contains(line, "SETTINGS-BLOCK") && !strings.Contains(line, "SECOND-LINE") {
			t.Fatalf("footer line %d is not the footer: %q", i, line)
		}
	}

	// the top left corner and every cell of the block are clickable
	for y := rect.Y; y < rect.Y+rect.Height; y++ {
		if !m.FooterAt(rect.X, y) {
			t.Errorf("the footer is not clickable at %d,%d", rect.X, y)
		}
	}

	if m.FooterAt(rect.X, rect.Y-1) {
		t.Error("the line above the footer must not be a part of it")
	}
	if m.FooterAt(rect.X-1, rect.Y) {
		t.Error("the border of the box must not be a part of the footer")
	}
}

func TestFooterDisappearsWithoutRoom(t *testing.T) {
	m := newTestModel(t, []string{"my wave"}, 32, 4)
	m.SetFooter("SETTINGS-BLOCK\nSECOND-LINE\nTHIRD-LINE")

	view := m.View()
	if !m.FooterRect().IsEmpty() {
		t.Fatalf("there is no room for the footer, but it was rendered: %+v", m.FooterRect())
	}
	if strings.Contains(strip(view), "SETTINGS-BLOCK") {
		t.Fatal("the footer was rendered without room for it")
	}
}

func TestFooterIsRemoved(t *testing.T) {
	m := newTestModel(t, []string{"my wave"}, 32, 20)
	m.SetFooter("SETTINGS-BLOCK")
	m.View()
	if m.FooterRect().IsEmpty() {
		t.Fatal("the footer was not set")
	}

	m.SetFooter("")
	m.View()
	if !m.FooterRect().IsEmpty() {
		t.Fatal("the removed footer still has an area")
	}
}
