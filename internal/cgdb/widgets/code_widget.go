package widgets

import (
	"math/rand"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/termui"
)

type CodeWidget struct {
	termui.BaseWidget

	Buffer   *core.Buffer
	Viewport core.Viewport
}

func NewCodeWidget() *CodeWidget {
	buf := core.NewBuffer()
	return &CodeWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Code"},
		Buffer:     buf,
		Viewport:   core.Viewport{Height: 10},
	}
}

func (m *CodeWidget) HandleEvent(ev tcell.Event) {
	switch ev.(type) {
	case *tcell.EventResize:
	case *tcell.EventKey:
	}
}

func (m *CodeWidget) Draw(c termui.Canvas) {
	style := tcell.StyleDefault

	bg := tcell.PaletteColor(rand.Intn(256))
	c.Fill(' ', style.Background(bg))
}
