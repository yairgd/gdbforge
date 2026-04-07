package termui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"
)

type Widget interface {
	HandleUIEvent(ev tcell.Event)
	SetSize(w int, h int)
	Draw(screen tcell.Screen)
	HandleCoreEvent(ev core.Event)
}
