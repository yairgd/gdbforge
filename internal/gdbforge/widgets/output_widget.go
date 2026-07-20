package widgets

import (
	"strings"
	"unicode"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
	"github.com/yairgd/gdbforge/internal/termui"
)

const outputTabWidth = 8

// OutputWidget is the IO console: inferior (program) stdout/stdin with
// terminal-like \n / \r / \t handling and ANSI colors. Built on ConsolePane
// (same scrollback path as GDB/Exec). Builtin name: :b io (:b output alias).
//
// When wired to an inferior PTY (-inferior-tty-set), program I/O goes through
// that master: AppendInferior for stdout, typed Enter → Send to stdin.
// GDB MI @"..." target-stream records are still accepted as a fallback.
// GDB console input never reaches this PTY — only this pane's input line does.
type OutputWidget struct {
	termui.BaseWidget
	console *termui.ConsolePane
	buf     *platform.Buffer

	miBuf           string // incomplete record across GDB PTY chunks
	inferiorRunning bool
	separateTTY     bool // true when using dedicated inferior PTY
	cur             []rune
	col             int

	inferior  *ptyx.TTY
	appState  *platform.AppState
	cancelSub func()

	lastRows, lastCols int
}

func NewOutputWidget() *OutputWidget {
	console := termui.NewConsolePane("IO")
	console.Prompt = ""
	console.SetANSI(true)
	console.SetInputEnabled(false)

	w := &OutputWidget{
		BaseWidget: termui.BaseWidget{PaneName: "IO"},
		console:    console,
		buf:        console.Buffer(),
	}
	w.initKeyBindings()
	w.ensureCurLine()
	return w
}

func (w *OutputWidget) initKeyBindings() {
	vp := w.console.Viewport()
	// Avoid Up/Down/Home/End — those belong to InputLine when stdin is enabled.
	w.BindKeyFunc("page-up", func(args ...any) { vp.ScrollPageUp(10) }, "<PgUp>")
	w.BindKeyFunc("page-down", func(args ...any) { vp.ScrollPageDown(10) }, "<PgDn>")
	w.BindKeyFunc("clear", func(args ...any) { w.Clear() }, "<C-l>")
}

// SetInferior wires the dedicated program PTY for stdin/stdout.
// Enables the input line; typed lines are written to the inferior master.
func (w *OutputWidget) SetInferior(tty *ptyx.TTY, state *platform.AppState) {
	w.inferior = tty
	w.appState = state
	w.separateTTY = tty != nil
	w.console.SetInputEnabled(tty != nil)
	if tty != nil {
		w.console.OnSubmit = w.onSubmit
		w.console.OnInterrupt = w.onInterrupt
		w.console.OnEOF = w.onEOF
	} else {
		w.console.OnSubmit = nil
		w.console.OnInterrupt = nil
		w.console.OnEOF = nil
	}
}

// Start bridges inferior PTY output into the UI event loop.
func (w *OutputWidget) Start(screen tcell.Screen) {
	if screen == nil || w.inferior == nil {
		return
	}
	if w.cancelSub != nil {
		w.cancelSub()
		w.cancelSub = nil
	}
	ch, cancel := w.inferior.Subscribe()
	w.cancelSub = cancel
	go func() {
		for msg := range ch {
			_ = screen.PostEvent(tcell.NewEventInterrupt(core.InferiorOutputMsg{
				Data: msg.Data,
				Err:  msg.Err,
			}))
		}
	}()
}

// StopInferior unsubscribes from the inferior PTY (session teardown).
func (w *OutputWidget) StopInferior() {
	if w.cancelSub != nil {
		w.cancelSub()
		w.cancelSub = nil
	}
}

// AppendPty ingests raw GDB PTY chunks: @" target streams, and (when no
// separate inferior TTY) raw text while the inferior is running.
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

// AppendInferior paints raw bytes from the program's dedicated PTY.
func (w *OutputWidget) AppendInferior(data string) {
	if data == "" {
		return
	}
	w.writeTarget(data)
	w.console.FollowTailAndScroll()
}

// AppendRaw is an alias for AppendPty (GDB PTY path).
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

	// Raw inferior text on the GDB PTY (shared-TTY fallback only).
	if !w.separateTTY && w.inferiorRunning {
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

func (w *OutputWidget) onSubmit(raw string) {
	if w.inferior == nil {
		return
	}
	line := raw
	if line == "" {
		line = w.console.Input().LastHistory()
	}
	if line != "" {
		w.console.Input().PushHistory(line)
	}
	// No EchoSubmit: the inferior TTY (ECHO) echoes when appropriate.
	send := func() { _ = w.inferior.Send(line) }
	if w.appState != nil {
		w.appState.WithPTYOwner(platform.PTYOwnerUI, send)
	} else {
		send()
	}
	w.console.Input().Clear()
	w.console.FollowTailAndScroll()
}

func (w *OutputWidget) onInterrupt() {
	if w.inferior == nil {
		return
	}
	send := func() { _ = w.inferior.SendRaw("\x03") }
	if w.appState != nil {
		w.appState.WithPTYOwner(platform.PTYOwnerUI, send)
	} else {
		send()
	}
}

func (w *OutputWidget) onEOF() {
	if w.inferior == nil {
		return
	}
	send := func() { _ = w.inferior.SendRaw("\x04") }
	if w.appState != nil {
		w.appState.WithPTYOwner(platform.PTYOwnerUI, send)
	} else {
		send()
	}
}

func (w *OutputWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *OutputWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventInterrupt:
		if data, ok := e.Data().(core.InferiorOutputMsg); ok && data.Data != "" {
			w.AppendInferior(data.Data)
		}
		return
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
	width, height := c.W(), c.H()
	if w.inferior != nil && width > 0 && height > 1 {
		rows := height - 1
		cols := width
		if rows != w.lastRows || cols != w.lastCols {
			w.lastRows = rows
			w.lastCols = cols
			_ = w.inferior.SetSize(uint16(rows), uint16(cols))
		}
	}
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
