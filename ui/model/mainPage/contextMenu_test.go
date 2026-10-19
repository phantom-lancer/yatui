package mainpage

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestContextMenuNavigation(t *testing.T) {
	cm := &contextMenu{
		items: []contextMenuItem{
			{label: "Play"},
			{label: "Start radio"},
			{label: "Add to playlist"},
		},
	}

	if cm.index != 0 {
		t.Errorf("initial index = %d, want 0", cm.index)
	}
	cm.up()
	if cm.index != 0 {
		t.Errorf("index after up from 0 = %d, want 0", cm.index)
	}
	cm.down()
	cm.down()
	if cm.index != 2 {
		t.Errorf("index after two downs = %d, want 2", cm.index)
	}
	cm.down()
	if cm.index != 2 {
		t.Errorf("index must stay at the last item, got %d", cm.index)
	}
}

func TestContextMenuHitTest(t *testing.T) {
	cm := &contextMenu{
		x: 10,
		y: 5,
		items: []contextMenuItem{
			{label: "Play"},
			{label: "Start radio"},
		},
	}

	w, h := cm.size()
	if !cm.contains(10, 5) {
		t.Error("top left corner must be inside")
	}
	if !cm.contains(10+w-1, 5+h-1) {
		t.Error("bottom right corner must be inside")
	}
	if cm.contains(10+w, 5) {
		t.Error("point past the right edge must be outside")
	}
	if cm.contains(10, 5+h) {
		t.Error("point past the bottom edge must be outside")
	}

	// rows inside the box but outside the items must not select anything
	if got := cm.itemAt(10, 5); got != -1 {
		t.Errorf("itemAt on the top border = %d, want -1", got)
	}

	// the first item is placed right below the top border
	if got := cm.itemAt(10, 5+1); got != 0 {
		t.Errorf("itemAt first row = %d, want 0", got)
	}
	if got := cm.itemAt(10, 5+2); got != 1 {
		t.Errorf("itemAt second row = %d, want 1", got)
	}
	if got := cm.itemAt(10, 5+h-1); got != -1 {
		t.Errorf("itemAt on the bottom border = %d, want -1", got)
	}
	if got := cm.itemAt(0, 0); got != -1 {
		t.Errorf("itemAt outside = %d, want -1", got)
	}
}

func TestContextMenuRenderOverlays(t *testing.T) {
	base := strings.Repeat("x", 40) + "\n" +
		strings.Repeat("y", 40) + "\n" +
		strings.Repeat("z", 40) + "\n" +
		strings.Repeat("w", 40)

	cm := &contextMenu{
		x: 5,
		y: 1,
		items: []contextMenuItem{
			{label: "Play"},
			{label: "Start radio"},
		},
	}

	got := cm.render(base)
	if got == base {
		t.Fatal("render must change the view")
	}

	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("render changed the line count: got %d want 4", len(lines))
	}

	// the menu box is 4 lines tall (2 borders + 2 items) and the base is
	// 4 lines tall as well, so it is shifted up to y=0:
	// line 0 - top border, line 1 - first item, line 2 - second item, line 3 - bottom border
	if !strings.Contains(strip(lines[1]), "Play") {
		t.Errorf("first menu item is missing: %q", strip(lines[1]))
	}
	if !strings.Contains(strip(lines[2]), "Start radio") {
		t.Errorf("second menu item is missing: %q", strip(lines[2]))
	}
}

func TestContextMenuClampsToScreen(t *testing.T) {
	base := strings.Repeat("x", 30) + "\n" + strings.Repeat("y", 30) + "\n" + strings.Repeat("z", 30)
	cm := &contextMenu{
		x:     28,
		y:     2,
		items: []contextMenuItem{{label: "Play"}},
	}

	got := cm.render(base)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("render changed the line count: got %d want 3", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > 30 {
			t.Errorf("line %d is wider than the screen: %d > 30", i, w)
		}
	}
}
