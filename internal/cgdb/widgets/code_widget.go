package widgets

import (
	"math/rand"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/termui"
)

type CodeWidget struct {
	Buffer   *core.Buffer
	Viewport core.Viewport

	InputBuf    string
	lastCommand string
	Cursor      int
}

func NewCodeWidget() *CodeWidget {
	buf := core.NewBuffer()
	return &CodeWidget{
		Buffer:   buf,
		Viewport: core.Viewport{Height: 10},
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

	title := "Status Line"
	for i, rr := range title {
		if i >= c.W() {
			break
		}
		c.SetContent(i, c.H(), rr, style)
	}
}
