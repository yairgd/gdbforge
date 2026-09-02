package widgets

import (
	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/ptyx"
	"github.com/yairgd/gdbforge/internal/termui"
)

const execScrollback = 4000

const execSessionEnded = "exec-session-ended"

// ExecWidget is the :! command shell terminal view.
type ExecWidget struct {
	termui.BaseWidget
	term      *termui.CompositeTerminal
	ended     bool
	onDismiss func()
}

func NewExecWidget() *ExecWidget {
	return &ExecWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Exec"},
		term:       termui.NewCompositeTerminalWithPrefix(80, 24, execScrollback, ""),
	}
}

func (m *ExecWidget) WireExec(tty *ptyx.TTY, onFrame func()) {
	if m == nil || m.term == nil {
		return
	}
	m.ended = false
	m.term.AttachTTY(tty, termui.WireTTYOpts{PostFrame: onFrame})
}

func (m *ExecWidget) SetOnDismiss(fn func()) { m.onDismiss = fn }

func (m *ExecWidget) NotifyExecEnded(screen tcell.Screen) {
	if screen == nil {
		return
	}
	_ = screen.PostEvent(tcell.NewEventInterrupt(execSessionEnded))
}

func (m *ExecWidget) Clear() {
	if m == nil {
		return
	}
	m.term.Close()
	m.term = termui.NewCompositeTerminalWithPrefix(80, 24, execScrollback, "")
	m.ended = false
}

func (m *ExecWidget) SetFocused(focused bool) {
	m.BaseWidget.SetFocused(focused)
}

func (m *ExecWidget) SetClipboard(io termui.ClipboardIO) {
	if m == nil || m.term == nil {
		return
	}
	m.term.SetClipboard(io)
}

func (m *ExecWidget) Draw(c termui.Canvas) {
	if m == nil {
		return
	}
	m.term.Paint(c, m.Focused())
}

func (m *ExecWidget) DrawStatusLine(c termui.Canvas, active bool) {
	m.BaseWidget.DrawStatusLine(c, active)
}

func (m *ExecWidget) markEnded() {
	if m == nil || m.ended {
		return
	}
	m.ended = true
	m.term.WriteRaw("\r\n[exec] process exited — press any key to return\r\n")
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

func (m *ExecWidget) Ended() bool { return m != nil && m.ended }

func (m *ExecWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	if m != nil && m.ended {
		m.dismiss()
		return true
	}
	if m != nil {
		return m.term.HandleKey(ev)
	}
	return false
}

func (m *ExecWidget) HandleEvent(ev tcell.Event) {
	if m == nil {
		return
	}
	switch e := ev.(type) {
	case *tcell.EventInterrupt:
		if s, ok := e.Data().(string); ok && s == execSessionEnded {
			m.markEnded()
			return
		}
	case *tcell.EventMouse:
		if m.term != nil {
			m.term.HandleMouse(e)
		}
	case *tcell.EventKey:
		if m.ended {
			m.dismiss()
			return
		}
		m.term.HandleKey(e)
	}
}
