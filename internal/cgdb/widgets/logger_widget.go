package widgets

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/termui"
)

type LoggerWidget struct {
	termui.BaseWidget

	Buffer   *core.Buffer
	Viewport core.Viewport
}

func NewLoggerWidget() *LoggerWidget {
	buf := core.NewBuffer()
	w := &LoggerWidget{
		BaseWidget: termui.NewBaseWidget(),
		Buffer:     buf,
		Viewport:   core.Viewport{Height: 10},
	}
	w.Start(func(ev termui.Event) {
		switch ev := ev.(type) {
		case termui.BaseEvent:
			w.Buffer.AppendText(ev.Cmd.Name)

		}
	})
	return w
}

func (m *LoggerWidget) HandleEvent(ev tcell.Event) {
	switch ev.(type) {
	case *tcell.EventResize:
	case *tcell.EventKey:
	}
}

func (m *LoggerWidget) Draw(c termui.Canvas) {
	style := tcell.StyleDefault
	title := "Logger"

	c.Printf(0, c.H(), style, "%s %d", title, 0)
}
