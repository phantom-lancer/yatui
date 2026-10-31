package tracker

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/phantom-lancer/yatui/config"
)

type helpKeyMap struct {
	PlayPause    key.Binding
	Stop         key.Binding
	PrevTrack    key.Binding
	NextTrack    key.Binding
	LikeUnlike   key.Binding
	CacheTrack   key.Binding
	Forward      key.Binding
	Backward     key.Binding
	VolUp        key.Binding
	VolDown      key.Binding
	ToggleLyrics key.Binding
	HidePlayer   key.Binding
}

func newHelpMap() *helpKeyMap {
	controls := config.Current.Controls
	return &helpKeyMap{
		PlayPause: key.NewBinding(
			controls.PlayerPause.Binding(),
			controls.PlayerPause.Help("играть/пауза"),
		),
		Stop: key.NewBinding(
			controls.PlayerStop.Binding(),
			controls.PlayerStop.Help("играть/пауза"),
		),
		PrevTrack: key.NewBinding(
			controls.PlayerPrevious.Binding(),
			controls.PlayerPrevious.Help("предыдущий трек"),
		),
		NextTrack: key.NewBinding(
			controls.PlayerNext.Binding(),
			controls.PlayerNext.Help("следующий трек"),
		),
		LikeUnlike: key.NewBinding(
			controls.PlayerLike.Binding(),
			controls.PlayerLike.Help("нравится"),
		),
		CacheTrack: key.NewBinding(
			controls.PlayerCache.Binding(),
			controls.PlayerCache.Help("кэшировать"),
		),
		Backward: key.NewBinding(
			controls.PlayerRewindBackward.Binding(),
			controls.PlayerRewindBackward.Help(fmt.Sprintf("−%d с", int(config.Current.RewindDuration))),
		),
		Forward: key.NewBinding(
			controls.PlayerRewindForward.Binding(),
			controls.PlayerRewindForward.Help(fmt.Sprintf("+%d с", int(config.Current.RewindDuration))),
		),
		VolUp: key.NewBinding(
			controls.PlayerVolUp.Binding(),
			controls.PlayerVolUp.Help("громче"),
		),
		VolDown: key.NewBinding(
			controls.PlayerVolDown.Binding(),
			controls.PlayerVolDown.Help("тише"),
		),
		ToggleLyrics: key.NewBinding(
			controls.PlayerToggleLyrics.Binding(),
			controls.PlayerToggleLyrics.Help("текст"),
		),
		HidePlayer: key.NewBinding(
			controls.PlayerHide.Binding(),
			controls.PlayerHide.Help("скрыть"),
		),
	}
}

func (k helpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.PlayPause, k.NextTrack, k.PrevTrack, k.LikeUnlike, k.VolUp, k.VolDown}
}

func (k helpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.PlayPause, k.LikeUnlike, k.ToggleLyrics, k.CacheTrack},
		{k.NextTrack, k.PrevTrack, k.Forward, k.Backward},
		{k.VolUp, k.VolDown, k.HidePlayer, k.Stop},
	}
}
