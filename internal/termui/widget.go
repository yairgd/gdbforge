package termui

import (
	"github.com/gdamore/tcell/v2"
)

type Widget interface {
	HandleEvent(ev tcell.Event)
	SetSize(w int, h int)
	//SetRect(r Rect)
	Parent() *Node
	SetParent(*Node)
	Draw(c Canvas)
}
