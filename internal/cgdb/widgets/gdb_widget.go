package widgets

import (
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/gdb"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

type GDBWidget struct {
	termui.BaseWidget

	out *termui.Viewport
	buf *platform.Buffer

	InputBuf string
	Cursor   int

	history   []string
	histIndex int // len(history) means editing a new/draft line
	histDraft string

	Debugger      core.Debugger
	gdbInputState gdb.GdbInputState
}

func NewGDBWidget(dbg core.Debugger) *GDBWidget {
	buf := platform.NewBuffer()
	out := termui.NewViewport(buf)
	out.SetFollowTail(true)
	out.SetReadOnly(true)
	out.SetCursorVisible(false)
	out.LineStyle = gdbLineStyle

	w := &GDBWidget{
		BaseWidget:    termui.BaseWidget{PaneName: "GDB"},
		out:           out,
		buf:           buf,
		Debugger:      dbg,
		Cursor:        0,
		histIndex:     0,
		gdbInputState: *gdb.NewGdbInputState(),
	}
	w.SetCursor(termui.NewNativeCursor())
	w.initKeyBindings()
	return w
}

func gdbLineStyle(line string) tcell.Style {
	st := tcell.StyleDefault
	if strings.HasPrefix(line, ">>>") {
		return st.Foreground(tcell.ColorTeal).Bold(true)
	}
	if strings.HasPrefix(line, "(gdb)") {
		return st.Foreground(tcell.ColorYellow)
	}
	return st
}

func (m *GDBWidget) initKeyBindings() {
	// History (readline / classic terminal)
	m.BindKeyFunc("hist-prev", func(args ...any) { m.historyPrev() }, "<Up>", "<C-p>")
	m.BindKeyFunc("hist-next", func(args ...any) { m.historyNext() }, "<Down>", "<C-n>")

	// Cursor motion
	m.BindKeyFunc("cursor-left", func(args ...any) { m.moveLeft() }, "<Left>", "<C-b>")
	m.BindKeyFunc("cursor-right", func(args ...any) { m.moveRight() }, "<Right>", "<C-f>")
	m.BindKeyFunc("cursor-home", func(args ...any) { m.moveHome() }, "<Home>", "<C-a>")
	m.BindKeyFunc("cursor-end", func(args ...any) { m.moveEnd() }, "<End>", "<C-e>")

	// Editing
	m.BindKeyFunc("backspace", func(args ...any) { m.backspace() }, "<Backspace>", "<C-h>")
	m.BindKeyFunc("delete", func(args ...any) { m.deleteForward() }, "<Delete>")
	m.BindKeyFunc("kill-line", func(args ...any) { m.killToEnd() }, "<C-k>")
	m.BindKeyFunc("kill-bol", func(args ...any) { m.killToStart() }, "<C-u>")
	m.BindKeyFunc("kill-word", func(args ...any) { m.killWord() }, "<C-w>")

	// Submit / signals / viewport
	m.BindKeyFunc("submit", func(args ...any) { m.submit() }, "<Enter>", "<C-m>", "<C-j>")
	m.BindKeyFunc("interrupt", func(args ...any) { m.interrupt() }, "<C-c>")
	m.BindKeyFunc("eof-quit", func(args ...any) { m.eofQuit() }, "<C-d>")
	m.BindKeyFunc("clear", func(args ...any) { m.Clear() }, "<C-l>")
	m.BindKeyFunc("scroll-up", func(args ...any) { m.out.ScrollPageUp(10) }, "<PgUp>")
	m.BindKeyFunc("scroll-down", func(args ...any) { m.out.ScrollPageDown(10) }, "<PgDn>")
}

func (m *GDBWidget) SetClipboard(io termui.ClipboardIO) {
	m.out.SetClipboard(io)
}

func (m *GDBWidget) StartGdbUIBridge(
	screen tcell.Screen,
	outputChan <-chan core.GdbOutputMsg,
) {
	if screen == nil {
		return
	}

	go func() {
		for msg := range outputChan {
			_ = screen.PostEvent(tcell.NewEventInterrupt(msg))
		}
		_ = screen.PostEvent(tcell.NewEventInterrupt("gdb-exit"))
	}()
}

func (m *GDBWidget) Clear() {
	if m.buf != nil {
		m.buf.Clear()
	}
	// Screen reset: empty buffer → prompt at top-left; as lines arrive the
	// prompt walks down one row at a time until the pane is full.
	m.out.Home()
	m.out.SetFollowTail(true)
	m.clearInputLine()
}

func (m *GDBWidget) SetFocused(focused bool) {
	m.BaseWidget.SetFocused(focused)
	// Caret stays on the (gdb) input line; output uses mouse selection only.
	m.out.SetCursorVisible(false)
}

func (m *GDBWidget) appendOutputLines(lines []string) {
	for _, line := range lines {
		m.appendOutputText(line)
	}
}

// appendOutputText mirrors the old core.Buffer AppendText "(gdb) " merge rule.
func (m *GDBWidget) appendOutputText(text string) {
	if n := m.buf.NumLines(); n > 0 && m.buf.Line(n-1) == "(gdb) " {
		m.buf.SetLine(n-1, m.buf.Line(n-1)+text)
		return
	}
	m.buf.AppendLine(text)
}

func (m *GDBWidget) stripTrailingBareGdbPrompt() {
	for m.buf.NumLines() > 0 {
		last := strings.TrimSpace(m.buf.Line(m.buf.NumLines() - 1))
		if last != "" && last != "(gdb)" {
			return
		}
		m.buf.RemoveLine(m.buf.NumLines() - 1)
		if last == "(gdb)" {
			return
		}
	}
}

func (m *GDBWidget) echoSubmittedCommand(cmd string) {
	n := m.buf.NumLines()
	if n > 0 && strings.TrimSpace(m.buf.Line(n-1)) == "(gdb)" {
		m.buf.SetLine(n-1, "(gdb) "+cmd)
		return
	}
	m.buf.AppendLine("(gdb) " + cmd)
}

func (m *GDBWidget) clearInputLine() {
	m.InputBuf = ""
	m.Cursor = 0
	m.histDraft = ""
	m.histIndex = len(m.history)
}

func (m *GDBWidget) handleStop(stop *gdb.MiStopMsg) {
	if stop == nil {
		return
	}
	switch stop.Reason {
	case "breakpoint-hit":
	case "end-stepping-range":
	case "exited-normally":
	}
}

func (m *GDBWidget) applyMiUpdate(upd gdb.MiUpdate) {
	if len(upd.DisplayLines) > 0 {
		m.appendOutputLines(upd.DisplayLines)
		// Prompt lives on the dedicated input row — don't leave a bare
		// "(gdb) " line in the scrollback above it.
		m.stripTrailingBareGdbPrompt()
		m.out.SetFollowTail(true)
		m.out.ScrollToBottom()
	}
	if upd.Stopped != nil {
		m.handleStop(upd.Stopped)
	}
}

func (m *GDBWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventInterrupt:
		switch data := e.Data().(type) {
		case core.GdbOutputMsg:
			if data.Data != "" {
				m.applyMiUpdate(m.gdbInputState.PushRaw(data.Data))
			}
		}

	case *tcell.EventMouse:
		m.out.HandleEvent(e)

	case *tcell.EventResize:
	case *tcell.EventKey:
		// Prefer copying a selection over sending SIGINT.
		if isCtrlC(e) && m.out.HasSelection() {
			m.out.CopySelection()
			return
		}
		if m.HandleBoundKey(e) {
			return
		}
		// Copy/cut/paste for output selection when not consumed as GDB editing.
		if isClipboardKey(e) {
			m.out.HandleEvent(e)
			return
		}
		// Some terminals still emit KeyBackspace (0x08) instead of KeyBackspace2.
		if e.Key() == tcell.KeyBackspace {
			m.backspace()
			return
		}
		if e.Key() == tcell.KeyRune {
			m.insertRune(e.Rune())
		}
	}
}

func isCtrlC(e *tcell.EventKey) bool {
	return e.Key() == tcell.KeyCtrlC ||
		(e.Key() == tcell.KeyRune && e.Rune() == 'c' && e.Modifiers()&tcell.ModCtrl != 0)
}

func isClipboardKey(e *tcell.EventKey) bool {
	if e.Key() == tcell.KeyCtrlC || e.Key() == tcell.KeyCtrlX || e.Key() == tcell.KeyCtrlV {
		return true
	}
	if e.Modifiers()&tcell.ModCtrl == 0 || e.Key() != tcell.KeyRune {
		return false
	}
	switch e.Rune() {
	case 'c', 'C', 'x', 'X', 'v', 'V':
		return true
	}
	return false
}

func (m *GDBWidget) insertRune(r rune) {
	s := string(r)
	m.InputBuf = m.InputBuf[:m.Cursor] + s + m.InputBuf[m.Cursor:]
	m.Cursor += len(s)
}

func (m *GDBWidget) moveLeft() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

func (m *GDBWidget) moveRight() {
	if m.Cursor < len(m.InputBuf) {
		m.Cursor++
	}
}

func (m *GDBWidget) moveHome() {
	m.Cursor = 0
}

func (m *GDBWidget) moveEnd() {
	m.Cursor = len(m.InputBuf)
}

func (m *GDBWidget) backspace() {
	if m.Cursor > 0 {
		m.InputBuf = m.InputBuf[:m.Cursor-1] + m.InputBuf[m.Cursor:]
		m.Cursor--
	}
}

func (m *GDBWidget) deleteForward() {
	if m.Cursor < len(m.InputBuf) {
		m.InputBuf = m.InputBuf[:m.Cursor] + m.InputBuf[m.Cursor+1:]
	}
}

func (m *GDBWidget) killToEnd() {
	if m.Cursor < len(m.InputBuf) {
		m.InputBuf = m.InputBuf[:m.Cursor]
	}
}

func (m *GDBWidget) killToStart() {
	if m.Cursor > 0 {
		m.InputBuf = m.InputBuf[m.Cursor:]
		m.Cursor = 0
	}
}

func (m *GDBWidget) killWord() {
	if m.Cursor == 0 {
		return
	}
	i := m.Cursor
	for i > 0 && m.InputBuf[i-1] == ' ' {
		i--
	}
	for i > 0 && m.InputBuf[i-1] != ' ' {
		i--
	}
	m.InputBuf = m.InputBuf[:i] + m.InputBuf[m.Cursor:]
	m.Cursor = i
}

func (m *GDBWidget) historyPrev() {
	if len(m.history) == 0 {
		return
	}
	if m.histIndex == len(m.history) {
		m.histDraft = m.InputBuf
	}
	if m.histIndex > 0 {
		m.histIndex--
		m.InputBuf = m.history[m.histIndex]
		m.Cursor = len(m.InputBuf)
	}
}

func (m *GDBWidget) historyNext() {
	if m.histIndex >= len(m.history) {
		return
	}
	m.histIndex++
	if m.histIndex == len(m.history) {
		m.InputBuf = m.histDraft
	} else {
		m.InputBuf = m.history[m.histIndex]
	}
	m.Cursor = len(m.InputBuf)
}

func (m *GDBWidget) pushHistory(cmd string) {
	if cmd == "" {
		return
	}
	if n := len(m.history); n > 0 && m.history[n-1] == cmd {
		m.histIndex = n
		m.histDraft = ""
		return
	}
	m.history = append(m.history, cmd)
	m.histIndex = len(m.history)
	m.histDraft = ""
}

func (m *GDBWidget) submit() {
	if m.Debugger == nil {
		return
	}
	cmd := m.InputBuf
	if cmd == "" && len(m.history) > 0 {
		cmd = m.history[len(m.history)-1]
	}
	if cmd != "" {
		m.pushHistory(cmd)
		m.echoSubmittedCommand(cmd)
		_ = m.Debugger.Send(cmd)
	} else {
		_ = m.Debugger.Send("")
	}
	// Always wipe the typed line after Enter — prompt stays, text does not.
	m.clearInputLine()
	m.out.SetFollowTail(true)
	m.out.ScrollToBottom()
}

func (m *GDBWidget) interrupt() {
	if m.Debugger != nil {
		_ = m.Debugger.SendRaw("\x03")
	}
}

func (m *GDBWidget) eofQuit() {
	if m.Debugger != nil {
		_ = m.Debugger.Send("q")
	}
}

func (m *GDBWidget) Draw(c termui.Canvas) {
	// TermApp does not wipe every frame — clear so the walking prompt and a
	// shorter InputBuf do not leave ghosts.
	for y := 0; y < c.H(); y++ {
		c.ClearLine(y, tcell.StyleDefault)
	}
	h := c.H()
	if h < 1 {
		return
	}

	n := 0
	if m.buf != nil {
		n = m.buf.NumLines()
	}

	// Terminal-style: prompt sits on the next free row under the scrollback.
	// Only pin it to the bottom (and scroll) once the pane is full.
	// While browsing history (follow-tail off), keep the prompt on the last row.
	inputY := n
	if !m.out.FollowTail() || inputY > h-1 {
		inputY = h - 1
	}
	if contentH := inputY; contentH > 0 {
		content := c.WithRect(c.ChildRect(0, 0, c.W(), contentH))
		m.out.Draw(content)
	}

	prompt := "(gdb) "
	promptLen := len(prompt)

	for x, ch := range prompt {
		if x >= c.W() {
			break
		}
		c.SetContent(x, inputY, ch, tcell.StyleDefault.Foreground(tcell.ColorYellow))
	}
	under := ' '
	for x, ch := range m.InputBuf {
		if x+promptLen >= c.W() {
			break
		}
		c.SetContent(x+promptLen, inputY, ch, tcell.StyleDefault)
		if x == m.Cursor {
			under = ch
		}
	}
	if m.Cursor < len(m.InputBuf) {
		under = rune(m.InputBuf[m.Cursor])
	}
	m.PaintCursor(c, m.Cursor+promptLen, inputY, under)
}
