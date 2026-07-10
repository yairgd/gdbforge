package widgets

import (
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

type LoggerWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer
	log      *platform.NamedLogger
}

func (w *LoggerWidget) Write(entry platform.LogEntry) error {
	w.viewport.Buffer.AppendLine(entry.Text)
	if w.viewport.FollowTail() {
		w.viewport.ScrollToBottom()
	}
	return nil
}

func NewLoggerWidget(ctx platform.AppContext) *LoggerWidget {
	buf := platform.NewBuffer()

	w := &LoggerWidget{
		BaseWidget: termui.NewBaseWidget(ctx),
		viewport:   termui.NewViewport(buf),
		buf:        buf,
	}
	w.PaneName = "Log"

	w.viewport.SetFollowTail(true)
	ctx.Log.AddSink(w)
	w.log = ctx.Log.Named("LoggerWidget")

	if ctx.Bus != nil {
		platform.Subscribe(ctx.Bus, w.showCompletion)
	}

	return w
}

func (m *LoggerWidget) HandleEvent(ev tcell.Event) {
	m.viewport.HandleEvent(ev)
}

func (m *LoggerWidget) SetCopyToClipboard(fn func(string)) {
	m.viewport.SetCopyToClipboard(fn)
}

func (m *LoggerWidget) Draw(c termui.Canvas) {
	m.viewport.Draw(c)
}

func (m *LoggerWidget) showCompletion(msg termui.CompletionMsg) {
    if len(msg.Names) == 0 {
        m.log.Info("completions: (none)")
        return
    }
    m.log.Info("completions: " + strings.Join(msg.Names, "  "))
}

func (m *LoggerWidget) showCompletion11(msg termui.CompletionMsg) {
	text := "completions: (none)"
	if len(msg.Names) > 0 {
		text = "completions: " + strings.Join(msg.Names, "  ")
	}
	m.viewport.Buffer.AppendLine(text)
	if m.viewport.FollowTail() {
		m.viewport.ScrollToBottom()
	}
}
