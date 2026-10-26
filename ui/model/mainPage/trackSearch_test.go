package mainpage

import (
	"testing"

	"github.com/phantom-lancer/yatui/ui/components/tracker"
)

// TestTrackSearchFilters checks that typing narrows the list.
func TestTrackSearchFilters(t *testing.T) {
	m := &Model{width: 100, height: 30}
	m.tracker = &tracker.Model{}
	m.tracklist = testTracklist(t, []string{"Alpha song", "Beta song", "Gamma song"})

	if cmd := m.openTrackSearch(); cmd == nil && m.trackSearch == nil {
		t.Fatal("the search did not open")
	}
	if got := len(m.trackSearch.results); got != 3 {
		t.Fatalf("empty query shows %d tracks, want 3", got)
	}

	for _, r := range []string{"g", "a", "m"} {
		if _, handled := m.trackSearchControl(keyMsg(r)); !handled {
			t.Fatalf("key %q was not handled", r)
		}
	}

	if m.trackSearch.query != "gam" {
		t.Errorf("query is %q, want gam", m.trackSearch.query)
	}
	if got := len(m.trackSearch.results); got != 1 {
		t.Fatalf("the filter found %d tracks, want 1", got)
	}
	if idx := m.trackSearch.selected(); idx != 2 {
		t.Errorf("selected original index is %d, want 2", idx)
	}

	// esc closes the window
	m.trackSearchControl(keyMsg("esc"))
	if m.trackSearch != nil {
		t.Error("the search did not close on esc")
	}
}

// TestTrackSearchOpensFromTheSidebar makes sure the new footer halves do not
// overlap: the top half is search, the bottom half is settings.
func TestTrackSearchOpensFromTheSidebar(t *testing.T) {
	m := &Model{width: 100, height: 30}
	m.tracker = &tracker.Model{}
	m.tracklist = testTracklist(t, []string{"one"})
	m.playlists = testPlaylists(t, []string{"list"})
	m.playlists.SetFooter(m.settingsButtonView())
	_ = m.playlists.View()
	m.updateMouseLayout("", "", "", "")

	if m.mouse.search.IsEmpty() || m.mouse.settings.IsEmpty() {
		t.Fatal("the footer halves have no area")
	}
	if m.mouse.search.Y+m.mouse.search.Height > m.mouse.settings.Y {
		t.Error("the search and settings areas overlap")
	}

	m.mousePress(m.mouse.search.X+1, m.mouse.search.Y+1)
	if m.trackSearch == nil {
		t.Error("the search button did not open the search")
	}
}
