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

	w.initKeyBindings()
	return w
}

func (m *LoggerWidget) initKeyBindings() {
	m.BindKeyFunc("scroll-up", func(args ...any) { m.viewport.Up() }, "<Up>", "k")
	m.BindKeyFunc("scroll-down", func(args ...any) { m.viewport.Down() }, "<Down>", "j")
	m.BindKeyFunc("page-up", func(args ...any) { m.viewport.PageUp(10) }, "<PgUp>", "<C-b>")
	m.BindKeyFunc("page-down", func(args ...any) { m.viewport.PageDown(10) }, "<PgDn>", "<C-f>")
	m.BindKeyFunc("home", func(args ...any) { m.viewport.Home() }, "<Home>", "g")
	m.BindKeyFunc("end", func(args ...any) { m.viewport.End() }, "<End>", "G")
	m.BindKeyFunc("clear", func(args ...any) { m.Clear() }, "<C-l>")
}

func (m *LoggerWidget) Clear() {
	if m.viewport != nil && m.viewport.Buffer != nil {
		m.viewport.Buffer.Clear()
		m.viewport.Home()
	}
}

func (m *LoggerWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventKey:
		if m.HandleBoundKey(e) {
			return
		}
	}
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
