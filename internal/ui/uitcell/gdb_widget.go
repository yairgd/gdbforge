package uitcell

import (
	"log"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"
	"github.com/yairgd/promptcore/internal/gdb"
	"github.com/yairgd/promptcore/internal/termui"
)

//////////////////////////
// GDB WIDGET
//////////////////////////

type GDBWidget struct {
	termui.BaseWidget
	Buffer   *core.Buffer
	Viewport core.Viewport

	InputBuf    string
	lastCommand string
	Cursor      int

	Debugger core.Debugger
}

func NewGDBWidget(uiContext termui.UIContext) *GDBWidget {
	buf := core.NewBuffer()
	client, outputChan, err := gdb.NewGDBClient()
	if err != nil {
		log.Fatal(err)
	}
	//	defer client.Close()

	widget := &GDBWidget{
		BaseWidget: termui.NewBaseWidget(uiContext.Emit),
		Buffer:     buf,
		Viewport:   core.Viewport{Height: 10},
		Debugger:   client,
		Cursor:     0,
	}
	widget.StartGdbUIBridge(uiContext.Screen(), outputChan)
	return widget
}

func (m *GDBWidget) StartGdbUIBridge(
	screen tcell.Screen,
	outputChan <-chan core.GdbOutputMsg,
) {
	go func() {
		for msg := range outputChan {
			screen.PostEvent(tcell.NewEventInterrupt(msg))
		}

		screen.PostEvent(tcell.NewEventInterrupt("gdb-exit"))
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

func (m *GDBWidget) OnGDBOutput(data string) {
	lines := strings.Split(data, "\n")

	var consoleBuf strings.Builder
	var targetBuf strings.Builder
	//	var logBuf strings.Builder

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch {
		// --- Console stream (~"...") ---
		case strings.HasPrefix(line, "~\"") && strings.HasSuffix(line, "\""):
			text := gdb.DecodeMIString(line[2 : len(line)-1])
			text = gdb.ExpandTabs(text, 8)
			if text != m.lastCommand {
				consoleBuf.WriteString(text)
			}

		// --- Target output (@"...") ---
		case strings.HasPrefix(line, "@\"") && strings.HasSuffix(line, "\""):
			text := gdb.DecodeMIString(line[2 : len(line)-1])
			text = gdb.ExpandTabs(text, 8)
			targetBuf.WriteString(text)

		// --- Log stream (&"...") ---
		case strings.HasPrefix(line, "&\"") && strings.HasSuffix(line, "\""):
			text := gdb.DecodeMIString(line[2 : len(line)-1])
			consoleBuf.WriteString(text)
		//	m.Buffer.AppendText(text)

		// --- Result record (^done, ^error...) ---
		case strings.HasPrefix(line, "^"):
			if strings.HasPrefix(line, "^error") {
				msg := gdb.ExtractMIField(line, "msg")
				consoleBuf.WriteString(msg)

				//	if msg != "" {
				//		consoleBuf.WriteString("ERROR: " + msg + "\n")
				//	} else {
				//		consoleBuf.WriteString("ERROR\n")
				//	}
				//	return
			}

		//	m.handleResultRecord(line)

		// --- Async record (*stopped, =breakpoint...) ---
		case strings.HasPrefix(line, "*") || strings.HasPrefix(line, "="):
			m.handleAsyncRecord(line)
		//	consoleBuf.WriteString(msg)

		// --- Prompt ---
		case line == "(gdb)":
			consoleBuf.WriteString("(gdb) ")

		//	m.handlePrompt()

		default:
			// fallback (sometimes garbage / partial lines)
			//	consoleBuf.WriteString(line + "\n")
		}
	}

	// --- Update UI (only what you want visible) ---
	if consoleBuf.Len() > 0 {
		m.Buffer.AppendText(consoleBuf.String())
	}

	if targetBuf.Len() > 0 {
		// optional: separate pane later
		//		m.Buffer.AppendText(targetBuf.String())
	}

	// logs usually hidden (debug only)
	// if logBuf.Len() > 0 { ... }

	m.Viewport.FollowBottom(m.Buffer)
}

// ////////////////////////
// EVENTS
// ////////////////////////
func (m *GDBWidget) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {
	case *tcell.EventInterrupt:
		switch data := e.Data().(type) {

		case core.GdbOutputMsg:
			m.OnGDBOutput(data.Data) // ✔ רק כאן!
		}
	case *tcell.EventResize:
		w, h := e.Size()

		m.SetSize(w, h)

		m.Viewport.Height = h - 1

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
}

// ////////////////////////
// DRAW
// ////////////////////////
func (m *GDBWidget) Draw(screen tcell.Screen) {
	screen.Clear()
	w, h := m.Size()

	// --- OUTPUT ---
	lines := m.Viewport.VisibleLines(m.Buffer)
	for y, line := range lines {
		// IMPORTANT: Reset to default style at the start of every line
		lineStyle := tcell.StyleDefault

		if strings.HasPrefix(line, ">>>") {
			// Apply special hardware status style
			lineStyle = lineStyle.Foreground(tcell.ColorTeal).Bold(true)
		} else if strings.HasPrefix(line, "(gdb)") {
			// Optional: Make the prompt stand out in a different color
			lineStyle = lineStyle.Foreground(tcell.ColorYellow)
		}

		// We call DrawANSIText ONCE per line.
		// No need for the 'for x := range line' loop here.
		core.DrawANSIText(screen, 0, y, line, lineStyle, w)
	}

	// --- INPUT LINE ---
	inputY := h - 2
	prompt := "(gdb) "
	promptLen := len(prompt)

	// Draw the static prompt
	//	DrawANSIText(screen, 0, inputY, prompt, tcell.StyleDefault.Foreground(tcell.ColorYellow), w)

	// Draw the user's current input
	for x, ch := range m.InputBuf {
		if x+promptLen >= w {
			break
		}
		screen.SetContent(x+promptLen, inputY, ch, nil, tcell.StyleDefault)
	}

	// --- CURSOR ---
	// Match the cursor position to the prompt offset
	screen.ShowCursor(m.Cursor+promptLen, inputY)

	screen.Show()
}
func (w *GDBWidget) HandleEvent1(ev tcell.Event) {
	w.App().RequestRedraw()

	w.Emit(core.GdbOutputMsg{Data: "test"})

}
