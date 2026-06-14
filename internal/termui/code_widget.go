package termui

import (
	"math/rand"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"
)

//////////////////////////
// GDB WIDGET
//////////////////////////

type CodeWidget struct {
	Buffer   *core.Buffer
	Viewport core.Viewport

	InputBuf    string
	lastCommand string
	Cursor      int
}

func NewCodeWidget() *CodeWidget {
	buf := core.NewBuffer()

	widget := &CodeWidget{
		Buffer:   buf,
		Viewport: core.Viewport{Height: 10},
	}
	return widget
}

// ////////////////////////
// EVENTS
// ////////////////////////
func (m *CodeWidget) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {
	case *tcell.EventResize:

	case *tcell.EventKey:

		switch e.Key() {

		case tcell.KeyCtrlC:
		case tcell.KeyCtrlD:
		}
		return

	}
}

// ////////////////////////
// DRAW
// ////////////////////////
func (m *CodeWidget) Draw(c Canvas) {

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

	if false {

		c.SetContent(0, 0, '*', style)
		c.SetContent(c.W(), 0, '*', style)
		c.SetContent(0, c.H(), '*', style)
		c.SetContent(c.W(), c.H(), '*', style)

		title := "Code Widget"
		for i, rr := range title {
			if i >= c.W() {
				break
			}
			c.SetContent(i, 0, rr, style)
		}
	}
}
