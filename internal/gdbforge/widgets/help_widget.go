package widgets

import (
	_ "embed"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// HelpWidget is a scrollable Viewport user manual (:help / :b help).
type HelpWidget struct {
	termui.BaseWidget
	doc *termui.ScrollDocument
	buf *platform.Buffer
}

// NewHelpWidget caches the guide text into a read-only Viewport buffer.
func NewHelpWidget() *HelpWidget {
	buf := platform.NewBuffer()
	vp := termui.NewScrollDocument(buf)
	vp.SetFollowTail(false)
	vp.SetReadOnly(true)
	vp.SetCursorVisible(false)
	vp.LineStyle = helpLineStyle

	w := &HelpWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Help"},
		doc:   vp,
		buf:        buf,
	}
	for _, line := range buildHelpLines() {
		buf.AppendLine(line)
	}
	vp.Home()
	w.initKeyBindings()
	return w
}

func helpLineStyle(line string) tcell.Style {
	st := tcell.StyleDefault
	if line == "gdbforge — user manual" {
		return st.Foreground(tcell.ColorYellow).Bold(true)
	}
	if strings.HasPrefix(line, "===") {
		return st.Foreground(tcell.ColorTeal).Bold(true)
	}
	if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, " ") && line != "" {
		return st.Foreground(tcell.ColorGray)
	}
	return st
}

// helpText is the in-app user manual (:help / :b help).
// Full markdown twin: docs/USER_GUIDE.md. Lua reference: docs/LUA_API.md.
//
//go:embed help.txt
var helpText string

func buildHelpLines() []string {
	if helpText == "" {
		return nil
	}
	lines := strings.Split(helpText, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

func (w *HelpWidget) initKeyBindings() {
	w.BindKeyFunc("scroll-up", func(args ...any) { w.doc.ScrollLineUp() }, "<Up>", "k")
	w.BindKeyFunc("scroll-down", func(args ...any) { w.doc.ScrollLineDown() }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.doc.ViewScrollColLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.doc.ViewScrollColRight() }, "<Right>")
	w.BindKeyFunc("page-up", func(args ...any) { w.doc.ScrollPageUp(10) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { w.doc.ScrollPageDown(10) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("home", func(args ...any) { w.doc.ScrollHome() }, "<Home>", "g")
	w.BindKeyFunc("end", func(args ...any) { w.doc.ScrollEnd() }, "<End>", "G")
}

func (w *HelpWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *HelpWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		w.doc.HandleEvent(e)
	case *tcell.EventKey:
		if w.HandleBoundKey(e) {
			return
		}
		w.doc.HandleEvent(e)
	}
}

func (w *HelpWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
	w.doc.SetCursorVisible(false)
}

func (w *HelpWidget) SetClipboard(io termui.ClipboardIO) {
	w.doc.SetClipboard(io)
}

func (w *HelpWidget) Draw(c termui.Canvas) {
	w.doc.Draw(c)
}

func (w *HelpWidget) Viewport() *termui.ScrollDocument {
	return w.doc
}

func (w *HelpWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}
