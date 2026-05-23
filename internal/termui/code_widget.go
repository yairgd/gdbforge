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
func (m *CodeWidget) Draw() {

	screen := m.uiContext.Screen()

	style := tcell.StyleDefault

	// Fill background
	for row := 0; row < m.rect.h; row++ {
		for col := 0; col < m.rect.w; col++ {
			screen.SetContent(
				m.rect.x+col,
				m.rect.y+row,
				' ',
				nil,
				style,
			)
		}
	}

	// Draw title
	title := "Code Widget"

	for i, rr := range title {

		if i >= m.rect.w {
			break
		}

		screen.SetContent(
			m.rect.x+i,
			m.rect.y,
			rr,
			nil,
			style,
		)
	}
}
