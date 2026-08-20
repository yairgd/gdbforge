package widgets

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/termui"
)

// LuaConsoleWidget is a line-oriented Lua REPL (ConsolePane chrome).
// The app owns the Runtime and eval policy; it paints via AppendOutput and
// handles OnSubmit / OnInterrupt intents.
type LuaConsoleWidget struct {
	console  *termui.ConsolePane
	handlers *ConsoleHandlers
}

// NewLuaConsoleWidget builds an empty Lua console view.
func NewLuaConsoleWidget() *LuaConsoleWidget {
	console := termui.NewConsolePane("Lua")
	console.Prompt = "lua> "
	console.PromptStyle = tcell.StyleDefault.Foreground(tcell.ColorGreen)
	w := &LuaConsoleWidget{console: console}
	w.EnsureLivePrompt()
	return w
}

func (m *LuaConsoleWidget) SetClipboard(io termui.ClipboardIO) {
	if m == nil || m.console == nil {
		return
	}
	m.console.SetClipboard(io)
}

// WireConsole attaches app handlers to this pane. nil clears handlers.
func (m *LuaConsoleWidget) WireConsole(h *ConsoleHandlers) {
	if m == nil || m.console == nil {
		return
	}
	m.handlers = h
	if h == nil {
		clearConsoleIntents(m.console)
		return
	}
	bindConsoleIntents(m.console, m.handleSubmit, m.handleInterrupt, m.handleEOF, m.handleSuspend)
}

func (m *LuaConsoleWidget) handleSubmit(cmd string) {
	if m.handlers != nil && m.handlers.Submit != nil {
		m.handlers.Submit(cmd)
	}
}

func (m *LuaConsoleWidget) handleInterrupt() {
	if m.handlers != nil && m.handlers.Interrupt != nil {
		m.handlers.Interrupt()
	}
}

func (m *LuaConsoleWidget) handleEOF() {
	if m.handlers != nil && m.handlers.EOF != nil {
		m.handlers.EOF()
	}
}

func (m *LuaConsoleWidget) handleSuspend() {
	if m.handlers != nil && m.handlers.Suspend != nil {
		m.handlers.Suspend()
	}
}

func (m *LuaConsoleWidget) AppendOutput(line string) {
	if m == nil || m.console == nil || line == "" {
		return
	}
	m.console.AppendScrollbackLine(line)
	m.console.FollowTailAndScroll()
}

func (m *LuaConsoleWidget) AppendLines(lines []string) {
	if m == nil || m.console == nil || len(lines) == 0 {
		return
	}
	m.console.AppendLines(lines)
	m.console.FollowTailAndScroll()
}

func (m *LuaConsoleWidget) Clear() {
	if m == nil || m.console == nil {
		return
	}
	m.console.Clear()
	m.EnsureLivePrompt()
}

func (m *LuaConsoleWidget) InputText() string {
	if m == nil || m.console == nil || m.console.Input() == nil {
		return ""
	}
	return m.console.Input().Text()
}

func (m *LuaConsoleWidget) LastHistory() string {
	if m == nil || m.console == nil || m.console.Input() == nil {
		return ""
	}
	return m.console.Input().LastHistory()
}

func (m *LuaConsoleWidget) PushHistory(cmd string) {
	if m == nil || m.console == nil || m.console.Input() == nil || cmd == "" {
		return
	}
	m.console.Input().PushHistory(cmd)
}

func (m *LuaConsoleWidget) EchoSubmit(cmd string) {
	if m == nil || m.console == nil {
		return
	}
	m.console.EchoSubmit(cmd)
}

func (m *LuaConsoleWidget) ApplyCompletion(name string) {
	if m == nil || m.console == nil || m.console.Input() == nil || name == "" {
		return
	}
	m.console.Input().SetText(name)
	m.console.ForceFollowTailAndScroll()
}

func (m *LuaConsoleWidget) InsertInputRune(r rune) {
	if m == nil || m.console == nil || m.console.Input() == nil {
		return
	}
	m.console.Input().InsertRune(r)
	m.console.ForceFollowTailAndScroll()
}

func (m *LuaConsoleWidget) BackspaceInput() {
	if m == nil || m.console == nil || m.console.Input() == nil {
		return
	}
	m.console.Input().Backspace()
	m.console.ForceFollowTailAndScroll()
}

func (m *LuaConsoleWidget) ClearInput() {
	if m == nil || m.console == nil || m.console.Input() == nil {
		return
	}
	m.console.Input().Clear()
}

func (m *LuaConsoleWidget) EnsureLivePrompt() {
	if m == nil || m.console == nil {
		return
	}
	m.console.EnsureLivePrompt()
}

func (m *LuaConsoleWidget) ForceFollowTailAndScroll() {
	if m == nil || m.console == nil {
		return
	}
	m.console.ForceFollowTailAndScroll()
}

func (m *LuaConsoleWidget) SetFocused(focused bool) {
	if m == nil || m.console == nil {
		return
	}
	m.console.SetFocused(focused)
}

func (m *LuaConsoleWidget) Draw(c termui.Canvas) {
	if m == nil || m.console == nil {
		return
	}
	m.console.Draw(c)
}

func (m *LuaConsoleWidget) Viewport() *termui.Viewport {
	if m == nil || m.console == nil {
		return nil
	}
	return m.console.Viewport()
}

func (m *LuaConsoleWidget) DrawStatusLine(c termui.Canvas, active bool) {
	if m == nil || m.console == nil {
		return
	}
	m.console.DrawStatusLine(c, active)
}

func (m *LuaConsoleWidget) HandleEvent(ev tcell.Event) {
	if m == nil || m.console == nil {
		return
	}
	m.console.HandleEvent(ev)
}
