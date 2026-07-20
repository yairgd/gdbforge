package termui

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbx/internal/platform"
)

// LoggerWidget is a reusable scrollable log pane. It implements platform.Sink
// so it can be registered on platform.Logger, and Clearable for :clear.
type LoggerWidget struct {
	BaseWidget
	viewport *Viewport
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
		BaseWidget: NewBaseWidget(ctx),
		viewport:   NewViewport(buf),
		buf:        buf,
	}
	w.PaneName = "Log"

	w.viewport.SetFollowTail(true)
	w.viewport.SetReadOnly(true)
	ctx.Log.AddSink(w)
	w.log = ctx.Log.Named("LoggerWidget")

	w.initKeyBindings()
	return w
}

func (m *LoggerWidget) initKeyBindings() {
	m.BindKeyFunc("scroll-up", func(args ...any) { m.viewport.ScrollLineUp() }, "<Up>", "k")
	m.BindKeyFunc("scroll-down", func(args ...any) { m.viewport.ScrollLineDown() }, "<Down>", "j")
	m.BindKeyFunc("scroll-left", func(args ...any) { m.viewport.ViewScrollColLeft() }, "<Left>")
	m.BindKeyFunc("scroll-right", func(args ...any) { m.viewport.ViewScrollColRight() }, "<Right>")
	m.BindKeyFunc("page-up", func(args ...any) { m.viewport.ScrollPageUp(10) }, "<PgUp>", "<C-b>")
	m.BindKeyFunc("page-down", func(args ...any) { m.viewport.ScrollPageDown(10) }, "<PgDn>", "<C-f>")
	m.BindKeyFunc("home", func(args ...any) { m.viewport.ScrollHome() }, "<Home>", "g")
	m.BindKeyFunc("end", func(args ...any) { m.viewport.ScrollEnd() }, "<End>", "G")
	m.BindKeyFunc("clear", func(args ...any) { m.Clear() }, "<C-l>")
}

func (m *LoggerWidget) Clear() {
	if m.viewport != nil && m.viewport.Buffer != nil {
		m.viewport.Buffer.Clear()
		m.viewport.Home() // top-left of the pane
		m.viewport.SetFollowTail(false)
	}
}

// HandleFocusKey handles scroll shortcuts when the Log pane is focused,
// including in normal mode (no need to press i first).
func (m *LoggerWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	if m.HandleBoundKey(ev) {
		return true
	}
	return false
}

func (m *LoggerWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		m.viewport.HandleEvent(e)
		return
	case *tcell.EventKey:
		if m.HandleBoundKey(e) {
			return
		}
		m.viewport.HandleEvent(e)
	}
}

// SetClipboard wires the shared Viewport copy/paste bridge.
func (m *LoggerWidget) SetClipboard(io ClipboardIO) {
	m.viewport.SetClipboard(io)
}

// SetCopyToClipboard keeps the older API; prefer SetClipboard.
func (m *LoggerWidget) SetCopyToClipboard(fn func(string)) {
	m.viewport.SetCopyToClipboard(fn)
}

func (m *LoggerWidget) Viewport() *Viewport {
	return m.viewport
}

func (m *LoggerWidget) SetFocused(focused bool) {
	m.BaseWidget.SetFocused(focused)
	// Only the focused log pane owns the system caret.
	m.viewport.SetCursorVisible(focused)
}

func (m *LoggerWidget) Draw(c Canvas) {
	m.viewport.Draw(c)
}
