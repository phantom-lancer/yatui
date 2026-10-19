package tracklist

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/dece2183/yamusic-tui/api"
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

func newTestModel(t *testing.T, titles []string, width, height int) *Model {
	t.Helper()

	likes := map[string]bool{}
	cached := map[string]bool{}
	m := &Model{
		help:    help.New(),
		helpMap: newHelpMap(),
		Title:   "Tracks",
	}

	items := make([]list.Item, len(titles))
	for i, title := range titles {
		items[i] = Item{
			Track: &api.Track{
				Id:         fmt.Sprint(i + 1),
				Title:      title,
				DurationMs: 100000,
				Available:  true,
			},
			Artists: "some artist",
		}
	}

	m.list = list.New(items, ItemDelegate{likesMap: &likes, cacheMap: &cached, layout: &m.layout}, 512, 512)
	m.list.Styles.Title = style.TrackListTitleStyle
	m.list.SetShowHelp(false)
	m.help.Ellipsis = "…"
	m.SetSize(width, height)

	return m
}

func init() {
	if err := config.InitialLoad(); err != nil {
		panic(err)
	}
	style.Apply(config.Current.Style)
}

func TestRowsMatchRenderedView(t *testing.T) {
	titles := []string{
		"first track", "second track", "third track",
		"fourth track", "fifth track", "sixth track",
		"seventh track", "eighth track", "ninth track",
	}
	m := newTestModel(t, titles, 60, 24)
	m.list.Select(2)
	view := m.View()

	if len(m.layout.rows) == 0 {
		t.Fatal("no rows recorded")
	}

	for _, row := range m.layout.rows {
		if row.index >= len(titles) {
			t.Fatalf("row index %d out of range", row.index)
		}
		// the title may be placed on any line of the item block
		found := false
		for i := 0; i < row.height; i++ {
			line := strip(lineOf(t, view, row.y+i))
			if strings.Contains(line, titles[row.index]) {
				found = true
			}
			if got := m.ItemAt(10, row.y+i); got != row.index {
				t.Errorf("ItemAt(y=%d) = %d, want %d", row.y+i, got, row.index)
			}
		}
		if !found {
			t.Errorf("row %d (y=%d, h=%d) does not contain %q", row.index, row.y, row.height, titles[row.index])
		}

		// the line right below the block belongs to another item
		if got := m.ItemAt(10, row.y+row.height); got != -1 && got != row.index+1 {
			t.Errorf("ItemAt below row %d = %d", row.index, got)
		}
	}

	// box border, title and status bar are not clickable
	for _, y := range []int{0, 1, 2, 3, 4, 5} {
		if got := m.ItemAt(10, y); got != -1 {
			t.Errorf("ItemAt(header y=%d) = %d, want -1", y, got)
		}
	}
	if got := m.ItemAt(0, 6); got != -1 {
		t.Errorf("ItemAt(border x=0) = %d, want -1", got)
	}
}

func TestLikeAt(t *testing.T) {
	titles := []string{"first track", "second track", "third track"}
	m := newTestModel(t, titles, 60, 24)
	m.list.Select(1)
	view := m.View()

	found := 0
	for _, row := range m.layout.rows {
		if row.like.IsEmpty() {
			t.Errorf("row %d has no like indicator", row.index)
			continue
		}
		found++
		if got := m.LikeAt(row.like.X, row.like.Y); got != row.index {
			t.Errorf("LikeAt(row %d) = %d", row.index, got)
		}
		if got := m.LikeAt(row.like.X+row.like.Width-1, row.like.Y); got != row.index {
			t.Errorf("LikeAt(right edge of row %d) = %d", row.index, got)
		}
		if got := m.LikeAt(row.like.X+row.like.Width, row.like.Y); got == row.index {
			t.Errorf("LikeAt past the like icon of row %d = %d", row.index, got)
		}
	}

	if found != len(m.layout.rows) {
		t.Errorf("found %d like indicators, want %d", found, len(m.layout.rows))
	}

	// the like icon is rendered exactly where it is reported
	for _, row := range m.layout.rows {
		line := strip(lineOf(t, view, row.like.Y))
		x := 0
		icon := -1
		for _, r := range line {
			if x == row.like.X {
				icon = 1
				break
			}
			x += runewidth.RuneWidth(r)
		}
		if icon < 0 {
			t.Errorf("like column %d is outside of the row line:\n%s", row.like.X, line)
		}
	}
}
