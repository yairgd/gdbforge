package widgets

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

type LoggerWidget struct {
	termui.BaseWidget

	Buffer   *core.Buffer
	Viewport core.Viewport
	log      *platform.NamedLogger
}

func (w *LoggerWidget) Write(entry platform.LogEntry) error {
	w.Buffer.AppendText(entry.Text)
	return nil
}

func NewLoggerWidget(logger *platform.Logger) *LoggerWidget {
	buf := core.NewBuffer()
	w := &LoggerWidget{
		BaseWidget: termui.NewBaseWidget(),
		Buffer:     buf,
		Viewport:   core.Viewport{Height: 10},
	}

	logger.AddSink(w)
	w.log = logger.Named("LoggerWidget")

	return w
}

func NewLoggerWidget1() *LoggerWidget {
	buf := core.NewBuffer()
	w := &LoggerWidget{
		BaseWidget: termui.NewBaseWidget(),
		Buffer:     buf,
		Viewport:   core.Viewport{Height: 10},
	}
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
