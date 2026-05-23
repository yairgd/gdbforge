package termui

import (
	"github.com/yairgd/promptcore/internal/core"
)

type BaseWidget struct {
	app       AppAPI
	uiContext UIContext
	rect      Rect
	parent    *Node
	width     int
	height    int
	Test      int
}

func (b *BaseWidget) SetParent(p *Node) {
	b.parent = p
}
func (b *BaseWidget) Parent() *Node {
	return b.parent
}
func (b *BaseWidget) Size() (int, int) {
	return b.width, b.height
}

func (b *BaseWidget) SetRect(r Rect) {
	b.rect = r
}

func (b *BaseWidget) SetSize(w, h int) {
	b.width = w
	b.height = h
}

func (b *BaseWidget) SetApp(app AppAPI) {
	b.app = app
}

func (b BaseWidget) App() AppAPI {
	return b.app
}

func (b *BaseWidget) Emit(e core.Event) {
	if b.uiContext.Emit != nil {
		b.uiContext.Emit(e)
	}
}

func NewBaseWidget(uiContext UIContext) BaseWidget {
	return BaseWidget{
		uiContext: uiContext,
	}
}
