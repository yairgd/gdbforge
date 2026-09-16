package widgets

import (
	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/termforge"
	"github.com/yairgd/termforge/ptyx"
)

const outputScrollback = 8000

// OutputWidget is the IO console: inferior PTY via xterm plus [lua] host lines.
type OutputWidget struct {
	termforge.BaseWidget
	term *termforge.CompositeTerminal
	clip termforge.TerminalClipboard
}

func NewOutputWidget() *OutputWidget {
	return &OutputWidget{
		BaseWidget: termforge.BaseWidget{PaneName: "IO"},
		term:       termforge.NewCompositeTerminal(80, 24, outputScrollback),
	}
}

func (w *OutputWidget) WireInferior(tty *ptyx.TTY, onFrame func()) {
	w.WireInferiorOpts(tty, termforge.WireTTYOpts{PostFrame: onFrame})
}

func (w *OutputWidget) WireInferiorOpts(tty *ptyx.TTY, opts termforge.WireTTYOpts) {
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
	w.term = termforge.NewCompositeTerminal(80, 24, outputScrollback)
	w.clip.Apply(w.term)
}

func (w *OutputWidget) SetClipboard(io termforge.ClipboardIO) {
	if w == nil {
		return
	}
	w.clip.Set(io)
	if w.term != nil {
		w.term.SetClipboard(io)
	}
}

func (w *OutputWidget) SetMouseOrigin(screenX, screenY int) {
	if w != nil && w.term != nil {
		w.term.SetMouseOrigin(screenX, screenY)
	}
}

func (w *OutputWidget) Draw(c termforge.Canvas) {
	if w == nil {
		return
	}
	w.term.Paint(c, w.Focused())
}

func (w *OutputWidget) DrawStatusLine(c termforge.Canvas, active bool) {
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
	case *tcell.EventClipboard:
		if w.term != nil {
			w.term.PasteBytes(e.Data())
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

func (w *OutputWidget) HasTerminalSelection() bool {
	return w != nil && w.term != nil && w.term.HasSelection()
}

func (w *OutputWidget) ResetTerminalInput() {
	if w != nil && w.term != nil {
		w.term.AfterHostResume()
	}
}
