package mainpage

import (
	"testing"

	"github.com/phantom-lancer/yatui/api"
)

func TestPinnedAlbumItemsKeepTracksSeparate(t *testing.T) {
	albums := []api.Album{
		{
			Title:   "First",
			Volumes: [][]api.Track{{testTrack("first-1", "First track")}},
		},
		{
			Title:   "Second",
			Volumes: [][]api.Track{{testTrack("second-1", "Second track")}},
		},
		{
			Title: "Empty",
		},
	}

	items := pinnedAlbumItems(albums)
	if len(items) != 2 {
		t.Fatalf("got %d pinned album items, want 2", len(items))
	}
	if items[0].Name != "First" || len(items[0].Tracks) != 1 || items[0].Tracks[0].Id != "first-1" {
		t.Fatalf("first pinned album contains unexpected tracks: %+v", items[0])
	}
	if items[1].Name != "Second" || len(items[1].Tracks) != 1 || items[1].Tracks[0].Id != "second-1" {
		t.Fatalf("second pinned album contains unexpected tracks: %+v", items[1])
	}
}
