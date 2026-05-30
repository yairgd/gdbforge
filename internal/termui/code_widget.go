package termui

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"
)

//////////////////////////
// GDB WIDGET
//////////////////////////

type CodeWidget struct {
	BaseWidget
	Buffer   *core.Buffer
	Viewport core.Viewport

	InputBuf    string
	lastCommand string
	Cursor      int
}

func NewCodeWidget(uiContext UIContext) *CodeWidget {
	buf := core.NewBuffer()
	//	defer client.Close()

	widget := &CodeWidget{
		BaseWidget: NewBaseWidget(uiContext),
		Buffer:     buf,
		Viewport:   core.Viewport{Height: 10},
	}
	return widget
}

// ////////////////////////
// EVENTS
// ////////////////////////
func (m *CodeWidget) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {
	case *tcell.EventResize:
		w, h := e.Size()

		m.SetSize(w, h)

		m.Viewport.Height = h - 1

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
func (m *CodeWidget) Draw(rect Rect) {

	screen := m.uiContext.Screen()

	style := tcell.StyleDefault

	if false {
		// Fill background
		for row := 1; row < rect.h-1; row++ {
			for col := 1; col < rect.w-1; col++ {
				screen.SetContent(
					rect.x+col,
					rect.y+row,
					' ',
					nil,
					style,
				)
			}
		}
	}
	// Draw title
	title := "Code Widget"

	screen.SetContent(
		rect.x,
		rect.y,
		'*',
		nil,
		style,
	)

	screen.SetContent(
		rect.x+rect.w-1,
		rect.y,
		'*',
		nil,
		style,
	)
	screen.SetContent(
		rect.x,
		rect.y+rect.h-1,
		'*',
		nil,
		style,
	)
	screen.SetContent(
		rect.x+rect.w-1,
		rect.y+rect.h-1,
		'*',
		nil,
		style,
	)

	for i, rr := range title {

		if i >= rect.w {
			break
		}

		screen.SetContent(
			rect.x+i+5,
			rect.y,
			rr,
			nil,
			style,
		)
	}
}
