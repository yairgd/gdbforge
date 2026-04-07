package termui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"
)

type AppAPI interface {
	GetScreen() tcell.Screen
	RequestRedraw()
	OpenWindow(w Widget)
	Publish(event core.Event)
}
