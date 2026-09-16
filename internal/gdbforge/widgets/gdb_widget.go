package widgets

import (
	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/termforge"
	"github.com/yairgd/termforge/ptyx"
)

const gdbScrollback = 8000

// debuggerPrompts are the CLI prompts GDB and Delve emit. Console detection
// drives Home/End line editing; Strip peels the prompt off the input line.
// "> " is GDB's continuation prompt inside define/commands/if blocks, where
// tab completion still has to see the bare input.
var debuggerPrompts = termforge.PromptPrefixes{
	Console: []string{"(gdb) ", "(dlv) "},
	Strip:   []string{"(gdb) ", "(dlv) ", "> "},
}

func newGDBTerminal() *termforge.CompositeTerminal {
	t := termforge.NewCompositeTerminalWithPrefix(80, 24, gdbScrollback, "")
	t.SetPromptPrefixes(debuggerPrompts)
	return t
}

// GDBWidget is the debugger CLI terminal (:b gdb).
type GDBWidget struct {
	termforge.BaseWidget
	term *termforge.CompositeTerminal
	clip termforge.TerminalClipboard
}

func NewGDBWidget() *GDBWidget {
	return &GDBWidget{
		BaseWidget: termforge.BaseWidget{PaneName: "GDB"},
		term:       newGDBTerminal(),
	}
}

func (w *GDBWidget) WireCLI(tty *ptyx.TTY, opts termforge.WireTTYOpts) {
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
	w.term = newGDBTerminal()
	w.clip.Apply(w.term)
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
	return termforge.InputLineText(w.term.Controller())
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
	termforge.ApplyCompletion(w.term.Controller(), cur, full)
}

func (w *GDBWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
}

func (w *GDBWidget) SetClipboard(io termforge.ClipboardIO) {
	if w == nil {
		return
	}
	w.clip.Set(io)
	if w.term != nil {
		w.term.SetClipboard(io)
	}
}

func (w *GDBWidget) SetMouseOrigin(screenX, screenY int) {
	if w != nil && w.term != nil {
		w.term.SetMouseOrigin(screenX, screenY)
	}
}

func (w *GDBWidget) Draw(c termforge.Canvas) {
	if w == nil {
		return
	}
	w.term.Paint(c, w.Focused())
}

func (w *GDBWidget) DrawStatusLine(c termforge.Canvas, active bool) {
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
	case *tcell.EventClipboard:
		if w.term != nil {
			w.term.PasteBytes(e.Data())
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

func (w *GDBWidget) HasTerminalSelection() bool {
	return w != nil && w.term != nil && w.term.HasSelection()
}

func (w *GDBWidget) ResetTerminalInput() {
	if w != nil && w.term != nil {
		w.term.AfterHostResume()
	}
}

// ScrollToBottom pins the viewport to the live tail (new PTY output visible).
func (w *GDBWidget) ScrollToBottom() {
	if w != nil && w.term != nil {
		w.term.ScrollToBottom()
	}
}
