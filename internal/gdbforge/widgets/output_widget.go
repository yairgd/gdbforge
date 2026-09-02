package widgets

import (
	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/ptyx"
	"github.com/yairgd/gdbforge/internal/termui"
)

const outputScrollback = 8000

// OutputWidget is the IO console: inferior PTY via xterm plus [lua] host lines.
type OutputWidget struct {
	termui.BaseWidget
	term *termui.CompositeTerminal
}

func NewOutputWidget() *OutputWidget {
	return &OutputWidget{
		BaseWidget: termui.BaseWidget{PaneName: "IO"},
		term:       termui.NewCompositeTerminal(80, 24, outputScrollback),
	}
}

func (w *OutputWidget) WireInferior(tty *ptyx.TTY, onFrame func()) {
	w.WireInferiorOpts(tty, termui.WireTTYOpts{PostFrame: onFrame})
}

func (w *OutputWidget) WireInferiorOpts(tty *ptyx.TTY, opts termui.WireTTYOpts) {
	if w == nil || w.term == nil {
		return
	}
	w.term.AttachTTY(tty, opts)
}

func (w *OutputWidget) Detach() {
	if w == nil || w.term == nil {
		return
	}
	w.term.Detach()
}

func (w *OutputWidget) AppendHostLine(s string) {
	if w == nil {
		return
	}
	w.term.WriteHostLine(s)
}

func (w *OutputWidget) Clear() {
	if w == nil {
		return
	}
	w.term.Close()
	w.term = termui.NewCompositeTerminal(80, 24, outputScrollback)
}

func (w *OutputWidget) SetClipboard(io termui.ClipboardIO) {
	if w == nil || w.term == nil {
		return
	}
	w.term.SetClipboard(io)
}

func (w *OutputWidget) Draw(c termui.Canvas) {
	if w == nil {
		return
	}
	w.term.Paint(c, w.Focused())
}

func (w *OutputWidget) DrawStatusLine(c termui.Canvas, active bool) {
	w.BaseWidget.DrawStatusLine(c, active)
}

func (w *OutputWidget) HandleEvent(ev tcell.Event) {
	if w == nil {
		return
	}
	switch e := ev.(type) {
	case *tcell.EventMouse:
		if w.term != nil {
			w.term.HandleMouse(e)
		}
	case *tcell.EventKey:
		w.term.HandleKey(e)
	}
}

func (w *OutputWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	if w == nil {
		return false
	}
	return w.term.HandleKey(ev)
}

func (w *OutputWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
}
