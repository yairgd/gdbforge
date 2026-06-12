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
	//	defer client.Close()

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
	//	w, h := e.Size()

	//	m.SetSize(w, h)

	//	m.Viewport.Height = h - 1

	case *tcell.EventKey:

		switch e.Key() {

		case tcell.KeyCtrlC:
		//	if m.Debugger.SendRaw != nil {
		//		m.Debugger.SendRaw("\x03") // SIGINT
		//	}
		//	return
		case tcell.KeyCtrlD:
			//	if m.Debugger.Send != nil {
			//		m.Debugger.Send("q\n") // SIGINT
			//	}
		}
		return

	}
}

// ////////////////////////
// DRAW
// ////////////////////////
func (m *CodeWidget) Draw(c Canvas) {

	//	screen := m.uiContext.Screen()

	style := tcell.StyleDefault

	// Fill background
	bg := tcell.PaletteColor(rand.Intn(256))

	for row := 0; row < c.rect.h; row++ {
		for col := 0; col < c.rect.w; col++ {

			c.Screen().SetContent(
				c.rect.x+col,
				c.rect.y+row,
				' ',
				nil,
				style.Background(bg),
			)
		}
	}

	title := "Status Line"
	for i, rr := range title {

		if i >= c.rect.w {
			break
		}

		c.Screen().SetContent(
			c.rect.x+i,
			c.rect.y+c.rect.h,
			rr,
			nil,
			style,
		)
	}

	if false {

		c.Screen().SetContent(
			c.rect.x,
			c.rect.y,
			'*',
			nil,
			style,
		)

		c.Screen().SetContent(
			c.rect.x+c.rect.w,
			c.rect.y,
			'*',
			nil,
			style,
		)
		c.Screen().SetContent(
			c.rect.x,
			c.rect.y+c.rect.h,
			'*',
			nil,
			style,
		)
		c.Screen().SetContent(
			c.rect.x+c.rect.w,
			c.rect.y+c.rect.h,
			'*',
			nil,
			style,
		)

		// Draw title
		title := "Code Widget"
		for i, rr := range title {

			if i >= c.rect.w {
				break
			}

			c.Screen().SetContent(
				c.rect.x+i,
				c.rect.y,
				rr,
				nil,
				style,
			)
		}
	}
}
