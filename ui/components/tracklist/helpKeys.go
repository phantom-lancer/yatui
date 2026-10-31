package tracklist

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/phantom-lancer/yatui/config"
)

type helpKeyMap struct {
	CursorUp           key.Binding
	CursorDown         key.Binding
	PageUp             key.Binding
	PageDown           key.Binding
	Play               key.Binding
	LikeUnlike         key.Binding
	ShowArtist         key.Binding
	AddToPlaylist      key.Binding
	RemoveFromPlaylist key.Binding
	Search             key.Binding
	Share              key.Binding
	Shuffle            key.Binding
	Back               key.Binding
	Reload             key.Binding
	ShowHelp           key.Binding
	CloseHelp          key.Binding
	HideTracklist      key.Binding

	HasArtist  bool
	Shafflable bool
}

func newHelpMap() *helpKeyMap {
	controls := config.Current.Controls
	return &helpKeyMap{
		CursorUp:           key.NewBinding(controls.CursorUp.Binding(), controls.CursorUp.Help("вверх")),
		CursorDown:         key.NewBinding(controls.CursorDown.Binding(), controls.CursorDown.Help("вниз")),
		PageUp:             key.NewBinding(controls.TracksNextPage.Binding(), controls.TracksNextPage.Help("стр. вверх")),
		PageDown:           key.NewBinding(controls.TracksPrevPage.Binding(), controls.TracksPrevPage.Help("стр. вниз")),
		Play:               key.NewBinding(controls.Apply.Binding(), controls.Apply.Help("играть")),
		LikeUnlike:         key.NewBinding(controls.TracksLike.Binding(), controls.TracksLike.Help("нравится")),
		ShowArtist:         key.NewBinding(controls.TracksArtist.Binding(), controls.TracksArtist.Help("артист")),
		AddToPlaylist:      key.NewBinding(controls.TracksAddToPlaylist.Binding(), controls.TracksAddToPlaylist.Help("в плейлист")),
		RemoveFromPlaylist: key.NewBinding(controls.TracksRemoveFromPlaylist.Binding(), controls.TracksRemoveFromPlaylist.Help("убрать")),
		Search:             key.NewBinding(controls.TracksSearch.Binding(), controls.TracksSearch.Help("поиск")),
		Share:              key.NewBinding(controls.TracksShare.Binding(), controls.TracksShare.Help("ссылка")),
		Shuffle:            key.NewBinding(controls.TracksShuffle.Binding(), controls.TracksShuffle.Help("перемешать")),
		Back:               key.NewBinding(controls.TracksBack.Binding(), controls.TracksBack.Help("назад")),
		Reload:             key.NewBinding(controls.Reload.Binding(), controls.Reload.Help("обновить")),
		HideTracklist:      key.NewBinding(controls.TracksHide.Binding(), controls.TracksHide.Help("скрыть")),
		ShowHelp:           key.NewBinding(controls.ShowAllKeys.Binding(), controls.ShowAllKeys.Help("все клавиши")),
		CloseHelp:          key.NewBinding(controls.ShowAllKeys.Binding(), controls.ShowAllKeys.Help("скрыть клавиши")),
	}
}

func (k helpKeyMap) ShortHelp() []key.Binding {
	if k.HasArtist {
		return []key.Binding{k.CursorUp, k.CursorDown, k.Play, k.LikeUnlike, k.ShowArtist, k.ShowHelp}
	}
	return []key.Binding{k.CursorUp, k.CursorDown, k.Play, k.LikeUnlike, k.ShowHelp}
}

func (k helpKeyMap) FullHelp() [][]key.Binding {
	bindings := [][]key.Binding{
		{k.CursorUp, k.CursorDown, k.PageUp, k.PageDown},
		{k.Play, k.LikeUnlike, k.AddToPlaylist, k.RemoveFromPlaylist},
		{k.Search, k.Share},
	}

	if k.HasArtist {
		bindings[2] = append(bindings[2], k.ShowArtist)
	}
	if k.Shafflable {
		bindings[2] = append(bindings[2], k.Shuffle)
	}

	return append(bindings, []key.Binding{k.Reload, k.HideTracklist, k.CloseHelp, k.Back})
}

func (k helpKeyMap) HelpHeight() int {
	maxLines := 0
	keys := k.FullHelp()
	for i := range keys {
		if len(keys[i]) > maxLines {
			maxLines = len(keys[i])
		}
	}
	return maxLines
}
