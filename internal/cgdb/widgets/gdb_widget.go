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

	InputBuf    string
	lastCommand string
	Cursor      int

	Debugger      core.Debugger
	gdbInputState gdb.GdbInputState
}

func NewGDBWidget(dbg core.Debugger) *GDBWidget {
	buf := core.NewBuffer()
	return &GDBWidget{
		BaseWidget:    termui.BaseWidget{PaneName: "GDB"},
		Buffer:        buf,
		Viewport:      core.Viewport{Height: 10},
		Debugger:      dbg,
		Cursor:        0,
		gdbInputState: *gdb.NewGdbInputState(),
	}
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
				lines := strings.Split(data.Data, "\n")
				for _, line := range lines {
					m.gdbInputState.PushLine(line)
				}
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
		if m.Debugger == nil {
			return
		}
		switch e.Key() {
		case tcell.KeyCtrlC:
			_ = m.Debugger.SendRaw("\x03")
		case tcell.KeyCtrlD:
			_ = m.Debugger.Send("q")
		case tcell.KeyEnter:
			if m.InputBuf != "" {
				_ = m.Debugger.Send(m.InputBuf)
				m.lastCommand = m.InputBuf
			} else if m.lastCommand != "" {
				_ = m.Debugger.Send(m.lastCommand)
			} else {
				_ = m.Debugger.Send("")
			}
			m.Viewport.FollowBottom(m.Buffer)
			m.InputBuf = ""
			m.Cursor = 0
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if m.Cursor > 0 {
				m.InputBuf = m.InputBuf[:m.Cursor-1] + m.InputBuf[m.Cursor:]
				m.Cursor--
			}
		case tcell.KeyLeft:
			if m.Cursor > 0 {
				m.Cursor--
			}
		case tcell.KeyRight:
			if m.Cursor < len(m.InputBuf) {
				m.Cursor++
			}
		case tcell.KeyUp:
			_ = m.Debugger.SendRaw("\x1b[A")
		case tcell.KeyDown:
			_ = m.Debugger.SendRaw("\x1b[B")
		case tcell.KeyRune:
			r := string(e.Rune())
			m.InputBuf = m.InputBuf[:m.Cursor] + r + m.InputBuf[m.Cursor:]
			m.Cursor += len(r)
		}
	}
}

func (m *GDBWidget) Draw(c termui.Canvas) {
	contentH := c.H() - 2
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

	inputY := c.H() - 2
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
	for x, ch := range m.InputBuf {
		if x+promptLen >= c.W() {
			break
		}
		c.SetContent(x+promptLen, inputY, ch, tcell.StyleDefault)
	}

	c.ShowNativeCursor(m.Cursor+promptLen, inputY)
}
