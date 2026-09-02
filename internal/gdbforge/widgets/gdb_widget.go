package widgets

import (
	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/ptyx"
	"github.com/yairgd/gdbforge/internal/termui"
)

const gdbScrollback = 8000

// GDBWidget is the debugger CLI terminal (:b gdb).
type GDBWidget struct {
	termui.BaseWidget
	term *termui.CompositeTerminal
}

func NewGDBWidget() *GDBWidget {
	return &GDBWidget{
		BaseWidget: termui.BaseWidget{PaneName: "GDB"},
		term:       termui.NewCompositeTerminalWithPrefix(80, 24, gdbScrollback, ""),
	}
}

func (w *GDBWidget) WireCLI(tty *ptyx.TTY, opts termui.WireTTYOpts) {
	if w == nil || w.term == nil {
		return
	}
	w.term.AttachTTY(tty, opts)
}

func (w *GDBWidget) WriteBoot(data string) {
	if w != nil && data != "" {
		w.term.WriteRaw(data)
	}
}

func (w *GDBWidget) AppendLines(lines []string) {
	for _, line := range lines {
		w.AppendHostLine(line)
	}
}

func (w *GDBWidget) AppendHostLine(s string) {
	if w == nil || s == "" {
		return
	}
	w.term.WriteHostLine(s)
}

func (w *GDBWidget) AppendTargetText(text string) {
	if w != nil && text != "" {
		w.term.WriteRaw(text)
	}
}

func (w *GDBWidget) Clear() {
	if w == nil {
		return
	}
	w.term.Close()
	w.term = termui.NewCompositeTerminalWithPrefix(80, 24, gdbScrollback, "")
}

func (w *GDBWidget) InsertInputRune(r rune) {
	if w != nil {
		_ = w.term.Controller().SendInput([]byte(string(r)))
	}
}

func (w *GDBWidget) BackspaceInput() {
	if w != nil {
		_ = w.term.Controller().SendInput([]byte("\x7f"))
	}
}

func (w *GDBWidget) InputText() string {
	if w == nil || w.term == nil {
		return ""
	}
	return termui.InputLineText(w.term.Controller())
}

func (w *GDBWidget) ApplyCompletion(full string) {
	w.ApplyCompletionFrom(w.InputText(), full)
}

// ApplyCompletionFrom inserts full using cur as the already-echoed input (avoids
// re-reading the xterm buffer while PTY echo is in flight).
func (w *GDBWidget) ApplyCompletionFrom(cur, full string) {
	if w == nil || full == "" {
		return
	}
	termui.ApplyCompletion(w.term.Controller(), cur, full)
}

func (w *GDBWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
}

func (w *GDBWidget) SetClipboard(io termui.ClipboardIO) {
	if w == nil || w.term == nil {
		return
	}
	w.term.SetClipboard(io)
}

func (w *GDBWidget) Draw(c termui.Canvas) {
	if w == nil {
		return
	}
	w.term.Paint(c, w.Focused())
}

func (w *GDBWidget) DrawStatusLine(c termui.Canvas, active bool) {
	w.BaseWidget.DrawStatusLine(c, active)
}

func (w *GDBWidget) HandleEvent(ev tcell.Event) {
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

func (w *GDBWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	if w == nil {
		return false
	}
	return w.term.HandleKey(ev)
}
