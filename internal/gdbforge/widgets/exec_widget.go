package widgets

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/termui"
)

// ExecWidget is a display-only Exec console view (ConsolePane chrome).
// The app owns ExecClient and send policy; it paints via Append/HandleEvent
// and handles OnSubmit / OnInterrupt / OnEOF intents.
type ExecWidget struct {
	termui.BaseWidget
	console   *termui.ConsolePane
	lineBuf   string
	pending   bool // last buffer line is an incomplete PTY line
	ended     bool // PTY/process exited; next key dismisses
	onClose   func()
	onDismiss func()
	handlers  *ConsoleHandlers

	lastRows, lastCols int
	setSize            func(rows, cols uint16) error
}

func NewExecWidget() *ExecWidget {
	console := termui.NewConsolePane("Exec")
	console.Prompt = "$ "
	console.PromptStyle = tcell.StyleDefault.Foreground(tcell.ColorGreen)
	console.SetANSI(true)

	w := &ExecWidget{BaseWidget: termui.BaseWidget{PaneName: "Exec"}, console: console}
	bindConsoleIntents(console, w.handleSubmit, w.handleInterrupt, w.handleEOF, w.handleSuspend)
	return w
}

func (m *ExecWidget) SetClipboard(io termui.ClipboardIO) {
	m.console.SetClipboard(io)
}

// SetSizeFunc registers a callback used when the pane size changes (PTY winsize).
func (m *ExecWidget) SetSizeFunc(fn func(rows, cols uint16) error) {
	m.setSize = fn
}

// WireConsole attaches app handlers to this pane. nil clears handlers.
func (m *ExecWidget) WireConsole(h *ConsoleHandlers) {
	m.handlers = h
}

// SetOnClose registers a callback invoked on Ctrl-D / EOF while the session is live
// when the app wants to close without forwarding ^D (legacy hook).
func (m *ExecWidget) SetOnClose(fn func()) {
	m.onClose = fn
}

// SetOnDismiss registers a callback invoked on the first key after the PTY exits
// (or after Ctrl-D closed the session), to leave the Exec pane.
func (m *ExecWidget) SetOnDismiss(fn func()) {
	m.onDismiss = fn
}

const execSessionEnded = "exec-session-ended"

// StartExecUIBridge posts PTY output and session-end onto the UI event loop.
func (m *ExecWidget) StartExecUIBridge(
	screen tcell.Screen,
	outputChan <-chan core.PtyOutputMsg,
) {
	if screen == nil {
		return
	}
	go func() {
		for msg := range outputChan {
			_ = screen.PostEvent(tcell.NewEventInterrupt(core.ExecOutputMsg{
				Data: msg.Data,
				Err:  msg.Err,
			}))
		}
		// Channel closed when the PTY reader finishes (process exit / Close).
		_ = screen.PostEvent(tcell.NewEventInterrupt(execSessionEnded))
	}()
}

func (m *ExecWidget) Clear() {
	m.console.Clear()
	m.lineBuf = ""
	m.pending = false
}

func (m *ExecWidget) SetFocused(focused bool) {
	m.console.SetFocused(focused)
}

func (m *ExecWidget) Draw(c termui.Canvas) {
	w, h := c.W(), c.H()
	if m.setSize != nil && (w != m.lastCols || h != m.lastRows) && w > 0 && h > 1 {
		m.lastCols = w
		m.lastRows = h
		_ = m.setSize(uint16(h-1), uint16(w))
	}
	m.console.Draw(c)
}

func (m *ExecWidget) DrawStatusLine(c termui.Canvas, active bool) {
	m.console.DrawStatusLine(c, active)
}

func (m *ExecWidget) handleSubmit(raw string) {
	cmd := raw
	if cmd == "" {
		cmd = m.console.Input().LastHistory()
	}
	if cmd != "" {
		m.console.Input().PushHistory(cmd)
	}
	if m.handlers != nil && m.handlers.Submit != nil {
		m.handlers.Submit(cmd)
	}
	m.console.Input().Clear()
	m.console.ForceFollowTailAndScroll()
}

func (m *ExecWidget) handleInterrupt() {
	if m.handlers != nil && m.handlers.Interrupt != nil {
		m.handlers.Interrupt()
	}
}

func (m *ExecWidget) handleEOF() {
	if m.ended {
		m.dismiss()
		return
	}
	if m.Ctx.Bus != nil {
		m.Publish(events.ExecClosedMsg{})
		return
	}
	if m.onClose != nil {
		m.onClose()
		return
	}
	if m.handlers != nil && m.handlers.EOF != nil {
		m.handlers.EOF()
	}
}

func (m *ExecWidget) handleSuspend() {
	if m.handlers != nil && m.handlers.Suspend != nil {
		m.handlers.Suspend()
	}
}

func (m *ExecWidget) markEnded() {
	if m.ended {
		return
	}
	m.ended = true
	m.lineBuf = ""
	m.pending = false
	m.console.SetLivePrompt(false)
	m.console.AppendText("[exec] process exited — press any key to return")
	m.console.ForceFollowTailAndScroll()
}

func (m *ExecWidget) dismiss() {
	if m.Ctx.Bus != nil {
		m.Publish(events.ExecDismissedMsg{})
		return
	}
	if m.onDismiss != nil {
		m.onDismiss()
	}
}

func (m *ExecWidget) Ended() bool { return m.ended }

func (m *ExecWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	if m.ended {
		m.dismiss()
		return true
	}
	return false
}

func (m *ExecWidget) pushRaw(data string) {
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '\n':
			m.commitLine()
		case '\r':
			// CR alone (not CR LF): redraw current line from column 0.
			if i+1 < len(data) && data[i+1] == '\n' {
				continue
			}
			m.lineBuf = ""
			m.syncPending()
		default:
			m.lineBuf += string(data[i])
		}
	}
	m.syncPending()
	m.console.FollowTailAndScroll()
}

func (m *ExecWidget) commitLine() {
	m.console.SetLivePrompt(false)
	buf := m.console.Buffer()
	if m.pending {
		n := buf.NumLines()
		if n > 0 {
			buf.SetLine(n-1, m.lineBuf)
		} else {
			buf.AppendLine(m.lineBuf)
		}
		m.pending = false
	} else {
		buf.AppendLine(m.lineBuf)
	}
	m.lineBuf = ""
}

func (m *ExecWidget) syncPending() {
	buf := m.console.Buffer()
	if m.lineBuf == "" && !m.pending {
		return
	}
	if m.pending {
		n := buf.NumLines()
		if n > 0 {
			buf.SetLine(n-1, m.lineBuf)
			m.console.SetLivePrompt(true)
			return
		}
	}
	buf.AppendLine(m.lineBuf)
	m.pending = true
	m.console.SetLivePrompt(true)
}

func (m *ExecWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventInterrupt:
		if s, ok := e.Data().(string); ok && s == execSessionEnded {
			m.markEnded()
			return
		}
		if data, ok := e.Data().(core.ExecOutputMsg); ok {
			if data.Err != nil {
				m.lineBuf = ""
				m.pending = false
				m.console.AppendText("[exec] " + data.Err.Error())
				m.console.FollowTailAndScroll()
			}
			if data.Data != "" {
				m.pushRaw(data.Data)
			}
		}
	case *tcell.EventKey:
		if m.ended {
			m.dismiss()
			return
		}
		m.console.HandleEvent(ev)
	default:
		if m.ended {
			return
		}
		m.console.HandleEvent(ev)
	}
}
