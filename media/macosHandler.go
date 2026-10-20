//go:build darwin && !nomedia

package media

import (
	"github.com/phantom-lancer/yatui/media/handler"
	"github.com/phantom-lancer/yatui/media/handler/macos"
)

func NewHandler(name, description string) handler.MediaHandler {
	return macos.NewHandler(name, description)
}
