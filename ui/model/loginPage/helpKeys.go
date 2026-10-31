package loginpage

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/phantom-lancer/yatui/config"
)

type helpKeyMap struct {
	apply key.Binding
	quit  key.Binding
}

func newHelpMap() *helpKeyMap {
	controls := config.Current.Controls
	return &helpKeyMap{
		apply: key.NewBinding(
			controls.Apply.Binding(),
			controls.Apply.Help("войти"),
		),
		quit: key.NewBinding(
			controls.Quit.Binding(),
			controls.Quit.Help("выход"),
		),
	}
}

func (k helpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.apply, k.quit}
}

func (k helpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		k.ShortHelp(),
	}
}
