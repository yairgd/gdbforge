package termui

import (
	"github.com/gdamore/tcell/v2"
)

type AppAPI interface {
	GetScreen() tcell.Screen
	RequestRedraw()
	OpenWindow(w Widget)
	Publish(event Event)
}

type UIContext interface {
	Emit(Event)
}
