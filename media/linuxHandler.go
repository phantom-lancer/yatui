//go:build linux && !nomedia

package media

import (
	"github.com/phantom-lancer/yatui/media/handler"
	"github.com/phantom-lancer/yatui/media/handler/mpris"
)

func NewHandler(name, description string) handler.MediaHandler {
	return mpris.NewHandler(name, description)
}
