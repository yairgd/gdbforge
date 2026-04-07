package termui

import (
	"github.com/gdamore/tcell/v2"
)

type Widget interface {
	HandleEvent(ev tcell.Event)
	SetSize(w int, h int)
	Draw(screen tcell.Screen)
}
