package termui

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

// LoggerWidget is a reusable scrollable log pane. It implements platform.Sink
// so it can be registered on platform.Logger, and Clearable for :clear.
type LoggerWidget struct {
	BaseWidget
	doc *ScrollDocument
	buf *platform.Buffer
	log *platform.NamedLogger
}

func (w *LoggerWidget) Write(entry platform.LogEntry) error {
	w.doc.Buffer.AppendLine(entry.Text)
	if w.doc.FollowTail() {
		w.doc.ScrollToBottom()
	}
	return nil
}

func NewLoggerWidget(ctx platform.AppContext) *LoggerWidget {
	buf := platform.NewBuffer()

	w := &LoggerWidget{
		BaseWidget: NewBaseWidget(ctx),
		doc:        NewScrollDocument(buf),
		buf:        buf,
	}
	w.PaneName = "Log"

	w.doc.SetFollowTail(true)
	w.doc.SetReadOnly(true)
	ctx.Log.AddSink(w)
	w.log = ctx.Log.Named("LoggerWidget")

	w.initKeyBindings()
	return w
}

func (m *LoggerWidget) initKeyBindings() {
	m.BindKeyFunc("scroll-up", func(args ...any) { m.doc.ScrollLineUp() }, "<Up>", "k")
	m.BindKeyFunc("scroll-down", func(args ...any) { m.doc.ScrollLineDown() }, "<Down>", "j")
	m.BindKeyFunc("scroll-left", func(args ...any) { m.doc.ViewScrollColLeft() }, "<Left>")
	m.BindKeyFunc("scroll-right", func(args ...any) { m.doc.ViewScrollColRight() }, "<Right>")
	m.BindKeyFunc("page-up", func(args ...any) { m.doc.ScrollPageUp(10) }, "<PgUp>", "<C-b>")
	m.BindKeyFunc("page-down", func(args ...any) { m.doc.ScrollPageDown(10) }, "<PgDn>", "<C-f>")
	m.BindKeyFunc("home", func(args ...any) { m.doc.ScrollHome() }, "<Home>", "g")
	m.BindKeyFunc("end", func(args ...any) { m.doc.ScrollEnd() }, "<End>", "G")
	m.BindKeyFunc("clear", func(args ...any) { m.Clear() }, "<C-l>")
}

func (m *LoggerWidget) Clear() {
	if m.doc != nil && m.doc.Buffer != nil {
		m.doc.Buffer.Clear()
		m.doc.Home()
		m.doc.SetFollowTail(false)
	}
}

func (m *LoggerWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return m.HandleBoundKey(ev)
}

func (m *LoggerWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		m.doc.HandleEvent(e)
	case *tcell.EventKey:
		if m.HandleBoundKey(e) {
			return
		}
		m.doc.HandleEvent(e)
	}
}

func (m *LoggerWidget) SetClipboard(io ClipboardIO) {
	m.doc.SetClipboard(io)
}

func (m *LoggerWidget) SetCopyToClipboard(fn func(string)) {
	m.doc.SetCopyToClipboard(fn)
}

func (m *LoggerWidget) ScrollDocument() *ScrollDocument {
	return m.doc
}

// Viewport returns the scroll document (legacy name for search/selection helpers).
func (m *LoggerWidget) Viewport() *ScrollDocument {
	return m.doc
}

func (m *LoggerWidget) SetFocused(focused bool) {
	m.BaseWidget.SetFocused(focused)
	m.doc.SetCursorVisible(focused)
}

func (m *LoggerWidget) Draw(c Canvas) {
	m.doc.Draw(c)
}
