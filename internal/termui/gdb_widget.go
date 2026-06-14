package termui

import (
	"log"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"
	"github.com/yairgd/promptcore/internal/gdb"
)

//////////////////////////
// GDB WIDGET
//////////////////////////

type GDBWidget struct {
	Buffer   *core.Buffer
	Viewport core.Viewport

	InputBuf    string
	lastCommand string
	Cursor      int

	Debugger      core.Debugger
	gdbInputState gdb.GdbInputState
}

func NewGDBWidget() *GDBWidget {
	buf := core.NewBuffer()
	client, outputChan, err := gdb.NewGDBClient()
	if err != nil {
		log.Fatal(err)
	}
	//	defer client.Close()

	widget := &GDBWidget{
		Buffer:        buf,
		Viewport:      core.Viewport{Height: 10},
		Debugger:      client,
		Cursor:        0,
		gdbInputState: *gdb.NewGdbInputState(),
	}
	widget.StartGdbUIBridge(nil /*uiContext.Screen()*/, outputChan)
	return widget
}

func (m *GDBWidget) StartGdbUIBridge(
	screen tcell.Screen,
	outputChan <-chan core.GdbOutputMsg,
) {
	go func() {
		for msg := range outputChan {
			//fmt.Print(msg.Data)
			//	if msg.Data == "(gdb) " {
			//	screen.PostEvent(tcell.NewEventInterrupt("gdb-timeout"))
			//	} else {
			screen.PostEvent(tcell.NewEventInterrupt(msg))
			//	}
		}
		screen.PostEvent(tcell.NewEventInterrupt("gdb-exit"))

	}()

	go func() {
		for {
			<-m.gdbInputState.Timer.C
			screen.PostEvent(tcell.NewEventInterrupt("gdb-timeout"))
		}
	}()
}

// ////////////////////////
// PUBLIC API
// ////////////////////////

func (m *GDBWidget) handleAsyncRecord(line string) {
	reason := gdb.ExtractMIField(line, "reason")

	if strings.HasPrefix(line, "*stopped") {

		switch reason {
		case "breakpoint-hit":
		//	m.Buffer.AppendText("\n🛑 Breakpoint hit\n")

		case "end-stepping-range":
			// step
		case "exited-normally":
			//			m.Buffer.AppendText("\nProgram exited\n")
		}
	}
	return

}

//func (m *GDBWidget) handlePrompt() {
//	m.Buffer.AppendText("(gdb) ")
//}

//func (m *GDBWidget) handleResultRecord(line string) {
//	if strings.HasPrefix(line, "^error") {
//		msg := gdb.ExtractMIField(line, "msg")
//		if msg != "" {
//			m.Buffer.AppendText("ERROR: " + msg + "\n")
//		} else {
//			m.Buffer.AppendText("ERROR\n")
//		}
//		return
//	}
//}

// ////////////////////////
// EVENTS
// ////////////////////////
func (m *GDBWidget) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {
	case *tcell.EventInterrupt:
		switch data := e.Data().(type) {

		case core.GdbOutputMsg:

			if data.Data != "" {
				lines := strings.Split(data.Data, "\n")
				for _, line := range lines {
					m.gdbInputState.PushLine(line)
					//consoleBuf = gdb.OnGDBOutput(line, m.lastCommand)

				}
			}
		case string:
			if data == "gdb-timeout" {
				buf := m.gdbInputState.Buffer()
				m.gdbInputState.Clear()
				miCmd := gdb.NewMiMsg(buf)
				//	if miCmd.GdbState() != gdb.Error {
				//	m.Buffer.AppendText(m.lastCommand)
				//	}
				m.Buffer.AppendBuffer(miCmd.CreateBufferForLine())
				m.Buffer.AppendText("(gdb) ")
				m.Viewport.FollowBottom(m.Buffer)

				// 👉 100ms passed without input
				//  if m.gdbInputState.IsTimeout(100 * time.Millisecond) {
				//    m.flushMIBlock() // <-- your function
				// }
			}
		}
		//	m.Viewport.FollowBottom(m.Buffer)

	case *tcell.EventResize:
		//	w, h := e.Size()

		//m.SetSize(w, h)

	//	m.Viewport.Height = h - 1

	case *tcell.EventKey:

		switch e.Key() {

		case tcell.KeyCtrlC:
			if m.Debugger.SendRaw != nil {
				m.Debugger.SendRaw("\x03") // SIGINT
			}
			return
		case tcell.KeyCtrlD:
			if m.Debugger.Send != nil {
				m.Debugger.Send("q\n") // SIGINT
			}
		//	return

		case tcell.KeyEnter:
			if m.Debugger.Send != nil {
				if m.InputBuf != "" {
					m.Debugger.Send(m.InputBuf)
					m.lastCommand = m.InputBuf
				} else {
					m.Debugger.Send(m.lastCommand)
				}
			}

			m.Viewport.FollowBottom(m.Buffer)

			m.InputBuf = ""
			m.Cursor = 0

		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if m.Cursor > 0 {
				m.InputBuf = m.InputBuf[:m.Cursor-0-1] + m.InputBuf[m.Cursor-0:]
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
			if m.Debugger.SendRaw != nil {
				m.Debugger.SendRaw("\x1b[A")
			}

		case tcell.KeyDown:
			if m.Debugger.SendRaw != nil {
				m.Debugger.SendRaw("\x1b[B")
			}

		case tcell.KeyRune:
			r := string(e.Rune())
			m.InputBuf =
				m.InputBuf[:m.Cursor-0] +
					r +
					m.InputBuf[m.Cursor-0:]
			m.Cursor += len(r)
		}
	}
	return
}

// ////////////////////////
// DRAW
// ////////////////////////
func (m *GDBWidget) Draw(c Canvas) {
	lines := m.Viewport.VisibleLines(m.Buffer)
	for y, line := range lines {
		lineStyle := tcell.StyleDefault

		if strings.HasPrefix(line, ">>>") {
			lineStyle = lineStyle.Foreground(tcell.ColorTeal).Bold(true)
		} else if strings.HasPrefix(line, "(gdb)") {
			lineStyle = lineStyle.Foreground(tcell.ColorYellow)
		}

		c.DrawANSIText(0, y, line, lineStyle)
	}

	inputY := c.H() - 2
	prompt := "(gdb) "
	promptLen := len(prompt)

	for x, ch := range m.InputBuf {
		if x+promptLen >= c.W() {
			break
		}
		c.SetContent(x+promptLen, inputY, ch, tcell.StyleDefault)
	}

	c.ShowCursor(m.Cursor+promptLen, inputY)
}
