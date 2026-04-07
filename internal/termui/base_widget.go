package termui

import (
	"github.com/yairgd/promptcore/internal/core"
)

type BaseWidget struct {
	app    AppAPI
	emit   core.Emitter
	width  int
	height int
	Test   int
}

func (b *BaseWidget) Size() (int, int) {
	return b.width, b.height
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
	if b.emit != nil {
		b.emit(e)
	}
}

func NewBaseWidget(emit core.Emitter) BaseWidget {
	return BaseWidget{
		emit: emit,
	}
}
