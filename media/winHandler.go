//go:build windows && !nomedia

package media

import (
	"github.com/phantom-lancer/yatui/media/handler"
	"github.com/phantom-lancer/yatui/media/handler/win"
)

func NewHandler(name, description string) handler.MediaHandler {
	return win.NewHandler(name, description)
}
