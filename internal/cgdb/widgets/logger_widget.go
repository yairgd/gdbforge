package widgets

import (
	"fmt"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

type LoggerWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer

	log *platform.NamedLogger
}

func (w *LoggerWidget) Write(entry platform.LogEntry) error {
	b := w.viewport.Buffer
	b.AppendText(entry.Text)
	return nil
}

func NewLoggerWidget(logger *platform.Logger) *LoggerWidget {
	buf := platform.NewBuffer()

	w := &LoggerWidget{
		BaseWidget: termui.NewBaseWidget(),
		viewport:   termui.NewViewport(buf),
	}

	logger.AddSink(w)
	w.log = logger.Named("LoggerWidget")
	for i := 0; i < 50; i++ {
		w.viewport.Buffer.AppendLine(fmt.Sprintf("Line number: %d", i))
	}

	return w
}

func (m *LoggerWidget) HandleEvent(ev tcell.Event) {
	m.viewport.HandleEvent(ev)
	switch ev.(type) {
	case *tcell.EventResize:
	case *tcell.EventKey:
	}
}

func (m *LoggerWidget) Draw(c termui.Canvas) {
	style := tcell.StyleDefault
	title := "Logger"
	c.Printf(0, c.H(), style, "%s %d", title, 0)
	m.viewport.Draw(c)

}
