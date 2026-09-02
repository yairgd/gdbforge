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
		term: termui.NewCompositeTerminalWithPrefix(80, 24, gdbScrollback, ""),
	}
}

func (w *GDBWidget) WireCLI(tty *ptyx.TTY, opts termui.WireTTYOpts) {
	if w == nil || w.term == nil {
		return
	}
	w.term.AttachTTY(tty, opts)
}

// WireConsole is deprecated; keys go through the terminal emulator.
func (w *GDBWidget) WireConsole(h *ConsoleHandlers) { _ = h }

func (w *GDBWidget) SetPromptStyleToken(token string) { _ = token }
func (w *GDBWidget) SetANSI(on bool)                  { _ = on }
func (w *GDBWidget) SetClipboard(io termui.ClipboardIO) { _ = io }

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
	w.term.WriteRaw(s + "\r\n")
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

func (w *GDBWidget) InputText() string            { return "" }
func (w *GDBWidget) LastHistory() string          { return "" }
func (w *GDBWidget) PushHistory(cmd string)         { _ = cmd }
func (w *GDBWidget) EchoSubmit(cmd string)          { _ = cmd }
func (w *GDBWidget) ClearInput()                    {}
func (w *GDBWidget) ApplyCompletion(name string) {
	if w != nil && name != "" {
		_ = w.term.Controller().SendString(name)
	}
}
func (w *GDBWidget) FollowTailAndScroll()       {}
func (w *GDBWidget) ForceFollowTailAndScroll()  {}
func (w *GDBWidget) LivePrompt() bool           { return false }
func (w *GDBWidget) SetLivePrompt(on bool)      { _ = on }
func (w *GDBWidget) BeginLiveHost(_ []string, _ string) {}
func (w *GDBWidget) AttachGdbPrompt(_ string)           {}
func (w *GDBWidget) StripTrailingGdbPrompt()              {}
func (w *GDBWidget) PaintMiDisplay(_ MiPaintUpdate, _, _ bool) {
}
func (w *GDBWidget) PaintDlvDisplay(_ []string, _ bool, _ string, _ bool) {}

func (w *GDBWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
}

func (w *GDBWidget) Draw(c termui.Canvas) {
	if w == nil {
		return
	}
	w.term.Paint(c, w.Focused())
}

func (w *GDBWidget) Viewport() *termui.Viewport { return nil }

func (w *GDBWidget) DrawStatusLine(c termui.Canvas, active bool) {
	w.BaseWidget.DrawStatusLine(c, active)
}

func (w *GDBWidget) HandleEvent(ev tcell.Event) {
	if w == nil {
		return
	}
	if e, ok := ev.(*tcell.EventKey); ok {
		w.term.HandleKey(e)
	}
}

func (w *GDBWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	if w == nil {
		return false
	}
	return w.term.HandleKey(ev)
}
