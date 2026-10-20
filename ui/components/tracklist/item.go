package tracklist

import (
	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/ui/helpers"
)

type Item struct {
	Track        *api.Track
	Album        *api.Album
	Artists      string
	IsPlaying    bool
	IsSuggestion bool
	// IsNext marks the tracks the user put in the queue: they are shown at
	// the top of the list and are not a part of the playlist.
	IsNext bool
}

func NewItem(track *api.Track) Item {
	return Item{
		Track:   track,
		Artists: helpers.ArtistList(track.Artists),
	}
}

func NewAlbumItem(album *api.Album) Item {
	return Item{
		Album:   album,
		Artists: helpers.ArtistList(album.Artists),
	}
}

func (i Item) FilterValue() string {
	if i.Track != nil {
		return i.Track.Title
	}
	if i.Album != nil {
		return i.Album.Title
	}
	return ""
}
