package widgets

import (
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/gdb"
	"github.com/yairgd/cgdb-go/internal/termui"
)

type GDBWidget struct {
	termui.BaseWidget

	Buffer   *core.Buffer
	Viewport core.Viewport

	InputBuf string
	Cursor   int

	history    []string
	histIndex  int // len(history) means editing a new/draft line
	histDraft  string

	Debugger      core.Debugger
	gdbInputState gdb.GdbInputState
}

func NewGDBWidget(dbg core.Debugger) *GDBWidget {
	buf := core.NewBuffer()
	w := &GDBWidget{
		BaseWidget:    termui.BaseWidget{PaneName: "GDB"},
		Buffer:        buf,
		Viewport:      core.Viewport{Height: 10},
		Debugger:      dbg,
		Cursor:        0,
		histIndex:     0,
		gdbInputState: *gdb.NewGdbInputState(),
	}
	w.SetCursor(termui.NewNativeCursor())
	w.initKeyBindings()
	return w
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
	m.BindKeyFunc("submit", func(args ...any) { m.submit() }, "<Enter>")
	m.BindKeyFunc("interrupt", func(args ...any) { m.interrupt() }, "<C-c>")
	m.BindKeyFunc("eof-quit", func(args ...any) { m.eofQuit() }, "<C-d>")
	m.BindKeyFunc("clear", func(args ...any) { m.Clear() }, "<C-l>")
	m.BindKeyFunc("scroll-up", func(args ...any) { m.scrollUp() }, "<PgUp>")
	m.BindKeyFunc("scroll-down", func(args ...any) { m.scrollDown() }, "<PgDn>")
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

	go func() {
		for {
			<-m.gdbInputState.Timer.C
			_ = screen.PostEvent(tcell.NewEventInterrupt("gdb-timeout"))
		}
	}()
}

func (m *GDBWidget) Clear() {
	if m.Buffer != nil {
		m.Buffer.Clear()
	}
	m.Viewport.TopLine = 0
	m.InputBuf = ""
	m.Cursor = 0
}

func (m *GDBWidget) handleAsyncRecord(line string) {
	reason := gdb.ExtractMIField(line, "reason")
	if strings.HasPrefix(line, "*stopped") {
		switch reason {
		case "breakpoint-hit":
		case "end-stepping-range":
		case "exited-normally":
		}
	}
}

func (m *GDBWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventInterrupt:
		switch data := e.Data().(type) {
		case core.GdbOutputMsg:
			if data.Data != "" {
				m.gdbInputState.PushRaw(data.Data)
			}
		case string:
			if data == "gdb-timeout" {
				buf := m.gdbInputState.Buffer()
				m.gdbInputState.Clear()
				miCmd := gdb.NewMiMsg(buf)
				m.Buffer.AppendBuffer(miCmd.CreateBufferForLine())
				m.Buffer.AppendText("(gdb) ")
				m.Viewport.FollowBottom(m.Buffer)
			}
		}

	case *tcell.EventResize:
	case *tcell.EventKey:
		if m.HandleBoundKey(e) {
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
		_ = m.Debugger.Send(cmd)
	} else {
		_ = m.Debugger.Send("")
	}
	m.Viewport.FollowBottom(m.Buffer)
	m.InputBuf = ""
	m.Cursor = 0
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

func (m *GDBWidget) scrollUp() {
	if m.Viewport.TopLine > 0 {
		m.Viewport.TopLine--
	}
}

func (m *GDBWidget) scrollDown() {
	if m.Buffer.NumLines() <= m.Viewport.Height {
		m.Viewport.TopLine = 0
		return
	}
	maxTop := m.Buffer.NumLines() - m.Viewport.Height
	if m.Viewport.TopLine < maxTop {
		m.Viewport.TopLine++
	}
}

func (m *GDBWidget) Draw(c termui.Canvas) {
	// Status bar is painted at y=c.H() (just below the leaf). Reserve only the
	// last in-pane row for the (gdb) input line — not two rows (that left a gap).
	contentH := c.H() - 1
	if contentH < 1 {
		contentH = 1
	}
	m.Viewport.Height = contentH

	lines := m.Viewport.VisibleLines(m.Buffer)
	for y, line := range lines {
		if y >= contentH {
			break
		}
		lineStyle := tcell.StyleDefault
		if strings.HasPrefix(line, ">>>") {
			lineStyle = lineStyle.Foreground(tcell.ColorTeal).Bold(true)
		} else if strings.HasPrefix(line, "(gdb)") {
			lineStyle = lineStyle.Foreground(tcell.ColorYellow)
		}
		c.DrawANSIText(0, y, line, lineStyle)
	}

	inputY := c.H() - 1
	if inputY < 0 {
		inputY = 0
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
