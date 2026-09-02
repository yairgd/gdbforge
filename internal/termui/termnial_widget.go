package termui

import (
	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/ptyx"
)

// TerminalWidget is a pane that shows one bidirectional PTY via CompositeTerminal.
type TerminalWidget struct {
	BaseWidget
	term *CompositeTerminal
}

func NewTerminalWidget(name string) *TerminalWidget {
	return &TerminalWidget{
		BaseWidget: BaseWidget{PaneName: name},
		term:       NewCompositeTerminal(80, 24, 8000),
	}
}

func NewTerminalWidgetWithScrollback(name string, scrollback int) *TerminalWidget {
	return &TerminalWidget{
		BaseWidget: BaseWidget{PaneName: name},
		term:       NewCompositeTerminal(80, 24, scrollback),
	}
}

func (w *TerminalWidget) Composite() *CompositeTerminal {
	if w == nil {
		return nil
	}
	return w.term
}

func (w *TerminalWidget) AttachTTY(tty *ptyx.TTY, opts WireTTYOpts) {
	if w == nil || w.term == nil {
		return
	}
	w.term.AttachTTY(tty, opts)
}

func (w *TerminalWidget) Detach() {
	if w == nil || w.term == nil {
		return
	}
	w.term.Detach()
}

func (w *TerminalWidget) WriteHostLine(s string) {
	if w != nil && w.term != nil {
		w.term.WriteHostLine(s)
	}
}

func (w *TerminalWidget) WriteRaw(data string) {
	if w != nil && w.term != nil {
		w.term.WriteRaw(data)
	}
}

func (w *TerminalWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	if w == nil || w.term == nil {
		return false
	}
	return w.term.HandleKey(ev)
}

func (w *TerminalWidget) HandleEvent(ev tcell.Event) {
	if e, ok := ev.(*tcell.EventKey); ok {
		w.HandleFocusKey(e)
	}
}

func (w *TerminalWidget) Draw(c Canvas) {
	if w == nil || w.term == nil {
		return
	}
	w.term.Paint(c, w.Focused())
}

func (w *TerminalWidget) DrawStatusLine(c Canvas, active bool) {
	w.BaseWidget.DrawStatusLine(c, active)
}

func (w *TerminalWidget) Viewport() *Viewport { return nil }

func (w *TerminalWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
}

func (w *TerminalWidget) SetClipboard(io ClipboardIO) { _ = io }
