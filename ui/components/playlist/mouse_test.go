package playlist

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/x/ansi"

	"github.com/dece2183/yamusic-tui/config"
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
	return lines[y]
}

func newTestModel(t *testing.T, names []string, width, height int) *Model {
	t.Helper()

	m := &Model{help: help.New(), helpMap: newHelpMap()}
	items := make([]list.Item, len(names))
	for i, name := range names {
		items[i] = &Item{Name: name, Kind: USER, Active: true}
	}

	m.list = list.New(items, ItemDelegate{layout: &m.layout}, 512, 512)
	m.list.Title = "YaMusic"
	m.list.SetShowStatusBar(false)
	m.list.SetShowHelp(false)
	m.SetSize(width, height)

	return m
}

func TestRowsMatchRenderedView(t *testing.T) {
	names := []string{
		"my wave", "liked tracks", "albums", "local files",
		"first playlist", "second playlist", "third playlist",
	}
	m := newTestModel(t, names, 32, 20)
	m.list.Select(2)
	view := m.View()

	if len(m.layout.rows) == 0 {
		t.Fatal("no rows recorded")
	}

	for _, row := range m.layout.rows {
		line := strip(lineOf(t, view, row.y))
		if !strings.Contains(line, names[row.index]) {
			t.Errorf("row %d (y=%d) does not contain %q:\n%s", row.index, row.y, names[row.index], line)
		}
		if got := m.ItemAt(5, row.y); got != row.index {
			t.Errorf("ItemAt(y=%d) = %d, want %d", row.y, got, row.index)
		}
	}

	// box border and title are not clickable
	for _, y := range []int{0, 1, 2, 3} {
		if got := m.ItemAt(5, y); got != -1 {
			t.Errorf("ItemAt(title y=%d) = %d, want -1", y, got)
		}
	}
	if got := m.ItemAt(0, 6); got != -1 {
		t.Errorf("ItemAt(border x=0) = %d, want -1", got)
	}
}

func TestClickAndCursor(t *testing.T) {
	names := []string{"my wave", "liked tracks", "albums", "local files"}
	m := newTestModel(t, names, 32, 20)
	m.View()

	if !m.ClickItem(2) {
		t.Fatal("ClickItem(2) failed")
	}
	if got := m.Index(); got != 2 {
		t.Errorf("index after click = %d, want 2", got)
	}
	if m.ClickItem(3) != true {
		t.Fatal("ClickItem(3) failed")
	}

	// inactive items are not selectable by click
	m.list.SetItem(1, &Item{Name: "header", Kind: NONE, Active: false})
	m.View()
	if m.ClickItem(1) {
		t.Error("inactive item must not be clickable")
	}

	// cursor helpers skip inactive items like the keyboard control does
	m.list.Select(0)
	if cmd := m.CursorDown(); cmd == nil {
		t.Error("CursorDown returned no command")
	}
	if got := m.Index(); got != 2 {
		t.Errorf("index after CursorDown = %d, want 2 (inactive item is skipped)", got)
	}
	if cmd := m.CursorUp(); cmd == nil {
		t.Error("CursorUp returned no command")
	}
	if got := m.Index(); got != 0 {
		t.Errorf("index after CursorUp = %d, want 0", got)
	}
}

func init() {
	if err := config.InitialLoad(); err != nil {
		panic(err)
	}
	style.Apply(config.Current.Style)
	_ = fmt.Sprint()
}
