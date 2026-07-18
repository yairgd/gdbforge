package widgets

import (
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/gdb"
	"github.com/yairgd/cgdb-go/internal/termui"
)

// GDBWidget is a ConsolePane wired to a GDB MI session (native REPL look).
// It owns the GDBClient lifetime: create via NewGDBWidget, start the UI
// bridge with Start, tear down with Close.
type GDBWidget struct {
	console       *termui.ConsolePane
	client        *gdb.GDBClient
	outputChan    <-chan core.GdbOutputMsg
	gdbInputState gdb.GdbInputState
}

func NewGDBWidget(gdbPath, prog string, args ...string) (*GDBWidget, error) {
	client, outputChan, err := gdb.NewGDBClient(gdbPath, prog, args...)
	if err != nil {
		return nil, err
	}

	console := termui.NewConsolePane("GDB")
	console.Prompt = "(gdb) "
	console.PromptStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow)
	console.LineStyle = gdbLineStyle

	w := &GDBWidget{
		console:       console,
		client:        client,
		outputChan:    outputChan,
		gdbInputState: *gdb.NewGdbInputState(),
	}

	console.OnSubmit = w.onSubmit
	console.OnInterrupt = w.onInterrupt
	console.OnEOF = w.onEOF
	return w, nil
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

// Session exposes the owned debugger for external APIs (e.g. MCP).
func (m *GDBWidget) Session() core.Session {
	return m.client
}

func (m *GDBWidget) Start(screen tcell.Screen) {
	if screen == nil || m.outputChan == nil {
		return
	}
	ch := m.outputChan
	go func() {
		for msg := range ch {
			_ = screen.PostEvent(tcell.NewEventInterrupt(msg))
		}
		_ = screen.PostEvent(tcell.NewEventInterrupt("gdb-exit"))
	}()
}

func (m *GDBWidget) Close() {
	if m.client != nil {
		m.client.Close()
		m.client = nil
	}
}

func (m *GDBWidget) SetClipboard(io termui.ClipboardIO) {
	m.console.SetClipboard(io)
}

func (m *GDBWidget) Clear() {
	m.console.Clear()
}

func (m *GDBWidget) SetFocused(focused bool) {
	m.console.SetFocused(focused)
}

func (m *GDBWidget) Draw(c termui.Canvas) {
	m.console.Draw(c)
}

func (m *GDBWidget) DrawStatusLine(c termui.Canvas, active bool) {
	m.console.DrawStatusLine(c, active)
}

func (m *GDBWidget) onSubmit(raw string) {
	if m.client == nil {
		return
	}
	cmd := raw
	if cmd == "" {
		cmd = m.console.Input().LastHistory()
	}
	if cmd != "" {
		m.console.Input().PushHistory(cmd)
		m.console.EchoSubmit(cmd)
		_ = m.client.Send(cmd)
	} else {
		_ = m.client.Send("")
	}
	m.console.Input().Clear()
	m.console.FollowTailAndScroll()
}

func (m *GDBWidget) onInterrupt() {
	if m.client != nil {
		_ = m.client.SendRaw("\x03")
	}
}

func (m *GDBWidget) onEOF() {
	if m.client != nil {
		_ = m.client.Send("q")
	}
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
		m.console.AppendLines(upd.DisplayLines)
		m.console.StripTrailingBarePrompt()
	}
	if upd.PromptReady {
		m.console.EnsureLivePrompt()
	}
	if len(upd.DisplayLines) > 0 || upd.PromptReady {
		m.console.FollowTailAndScroll()
	}
	if upd.Stopped != nil {
		m.handleStop(upd.Stopped)
	}
}

func (m *GDBWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventInterrupt:
		if data, ok := e.Data().(core.GdbOutputMsg); ok && data.Data != "" {
			m.applyMiUpdate(m.gdbInputState.PushRaw(data.Data))
		}
	default:
		m.console.HandleEvent(ev)
	}
}
