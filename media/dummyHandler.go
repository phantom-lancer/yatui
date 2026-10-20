//go:build nomedia

package media

import (
	"github.com/phantom-lancer/yatui/media/handler"
	"github.com/phantom-lancer/yatui/media/handler/dummy"
)

func NewHandler(name, description string) handler.MediaHandler {
	return dummy.NewHandler(name, description)
}
