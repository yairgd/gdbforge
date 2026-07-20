package widgets

import (
	"strings"
	"unicode"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbx/internal/gdb"
	"github.com/yairgd/gdbx/internal/platform"
	"github.com/yairgd/gdbx/internal/termui"
)

const outputTabWidth = 8

// OutputWidget shows inferior (program) stdout with terminal-like
// \n / \r / \t handling and ANSI colors. Built on a read-only ConsolePane
// (same scrollback path as GDB/Exec). GDB may deliver that as @"..." target-
// stream records and/or as raw PTY text while the inferior is running.
type OutputWidget struct {
	termui.BaseWidget
	console *termui.ConsolePane
	buf     *platform.Buffer

	miBuf           string // incomplete record across PTY chunks
	inferiorRunning bool
	cur             []rune
	col             int
}

func NewOutputWidget() *OutputWidget {
	console := termui.NewConsolePane("Output")
	console.Prompt = ""
	console.SetANSI(true)
	console.SetInputEnabled(false)

	w := &OutputWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Output"},
		console:    console,
		buf:        console.Buffer(),
	}
	w.initKeyBindings()
	w.ensureCurLine()
	return w
}

func (w *OutputWidget) initKeyBindings() {
	vp := w.console.Viewport()
	w.BindKeyFunc("scroll-up", func(args ...any) { vp.ScrollLineUp() }, "<Up>", "k")
	w.BindKeyFunc("scroll-down", func(args ...any) { vp.ScrollLineDown() }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { vp.ViewScrollColLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { vp.ViewScrollColRight() }, "<Right>")
	w.BindKeyFunc("page-up", func(args ...any) { vp.ScrollPageUp(10) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { vp.ScrollPageDown(10) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("home", func(args ...any) { vp.ScrollHome() }, "<Home>", "g")
	w.BindKeyFunc("end", func(args ...any) { vp.ScrollEnd() }, "<End>", "G")
	w.BindKeyFunc("clear", func(args ...any) { w.Clear() }, "<C-l>")
}

// AppendPty ingests raw GDB PTY chunks and paints inferior stdout only.
func (w *OutputWidget) AppendPty(data string) {
	if data == "" {
		return
	}
	w.miBuf += data
	for {
		i := strings.IndexByte(w.miBuf, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimRight(w.miBuf[:i], "\r")
		w.miBuf = w.miBuf[i+1:]
		w.consumeRecord(line)
	}
	w.console.FollowTailAndScroll()
}

// AppendRaw is an alias for AppendPty.
func (w *OutputWidget) AppendRaw(data string) {
	w.AppendPty(data)
}

func (w *OutputWidget) consumeRecord(line string) {
	trim := strings.TrimSpace(line)

	if isRunningRecord(trim) {
		w.inferiorRunning = true
		return
	}
	if isStoppedRecord(trim) {
		// Only end capture when the process exits; keep accepting stdout across
		// step/next stops so lines are not dropped, and ^running must not clear.
		if strings.Contains(trim, "exited") {
			w.inferiorRunning = false
		}
		return
	}

	if text, ok := decodeTargetStreamLine(trim); ok {
		w.writeTarget(text)
		return
	}

	if isMILine(trim) {
		return
	}

	// Raw inferior text (common when GDB shares the PTY with the program).
	if w.inferiorRunning {
		w.writeTarget(line + "\n")
	}
}

func isRunningRecord(line string) bool {
	return strings.HasPrefix(line, "^running") ||
		strings.HasPrefix(line, "*running") ||
		hasTokenPrefix(line, "^running") ||
		hasTokenPrefix(line, "*running")
}

func isStoppedRecord(line string) bool {
	return strings.HasPrefix(line, "*stopped") || hasTokenPrefix(line, "*stopped")
}

func hasTokenPrefix(line, kind string) bool {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	return i > 0 && strings.HasPrefix(line[i:], kind)
}

func isMILine(line string) bool {
	if line == "" || line == "(gdb)" {
		return true
	}
	if strings.HasPrefix(line, ">>>") {
		return true
	}
	switch line[0] {
	case '~', '@', '&', '^', '*', '=':
		return true
	}
	return hasTokenPrefix(line, "^") || hasTokenPrefix(line, "*") || hasTokenPrefix(line, "=")
}

func decodeTargetStreamLine(line string) (string, bool) {
	if !strings.HasPrefix(line, "@\"") || !strings.HasSuffix(line, "\"") || len(line) < 3 {
		return "", false
	}
	return gdb.DecodeMIString(line[2 : len(line)-1]), true
}

func (w *OutputWidget) writeTarget(text string) {
	for _, r := range text {
		switch r {
		case '\n':
			w.commitLine()
		case '\r':
			w.col = 0
		case '\t':
			spaces := outputTabWidth - (w.col % outputTabWidth)
			if spaces == 0 {
				spaces = outputTabWidth
			}
			for i := 0; i < spaces; i++ {
				w.putRune(' ')
			}
		case '\x1b':
			// Keep ESC so Viewport ANSI mode can colorize SGR sequences.
			w.putRune(r)
		default:
			if unicode.IsControl(r) {
				continue
			}
			w.putRune(r)
		}
	}
	w.syncCurLine()
}

func (w *OutputWidget) putRune(r rune) {
	w.ensureCurLine()
	for len(w.cur) < w.col {
		w.cur = append(w.cur, ' ')
	}
	if w.col < len(w.cur) {
		w.cur[w.col] = r
	} else {
		w.cur = append(w.cur, r)
	}
	w.col++
}

func (w *OutputWidget) ensureCurLine() {
	if w.buf.NumLines() == 0 {
		w.buf.AppendLine("")
	}
}

func (w *OutputWidget) syncCurLine() {
	w.ensureCurLine()
	idx := w.buf.NumLines() - 1
	w.buf.SetLine(idx, string(w.cur))
}

func (w *OutputWidget) commitLine() {
	w.syncCurLine()
	w.cur = w.cur[:0]
	w.col = 0
	w.buf.AppendLine("")
}

func (w *OutputWidget) Clear() {
	w.miBuf = ""
	w.cur = w.cur[:0]
	w.col = 0
	w.console.Clear()
	w.ensureCurLine()
	w.syncCurLine()
	w.console.FollowTailAndScroll()
}

func (w *OutputWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *OutputWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventKey:
		if w.HandleBoundKey(e) {
			return
		}
	}
	w.console.HandleEvent(ev)
}

func (w *OutputWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
	w.console.SetFocused(focused)
}

func (w *OutputWidget) SetClipboard(io termui.ClipboardIO) {
	w.console.SetClipboard(io)
}

func (w *OutputWidget) Draw(c termui.Canvas) {
	w.console.Draw(c)
}

func (w *OutputWidget) DrawStatusLine(c termui.Canvas, active bool) {
	w.console.DrawStatusLine(c, active)
}

func (w *OutputWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, w.buf.Line(i))
	}
	if len(out) > 0 && out[len(out)-1] == "" && len(w.cur) == 0 {
		out = out[:len(out)-1]
	}
	return out
}
