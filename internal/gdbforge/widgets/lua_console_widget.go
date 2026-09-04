package widgets

import (
	"strings"
	"unicode/utf8"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/termui"
)

const (
	luaPrompt     = "lua> "
	luaScrollback = 8000
)

// LuaConsoleWidget is a line-oriented Lua REPL backed by xterm (no PTY).
// The app owns the Runtime and eval policy; it paints via AppendOutput and
// handles Submit / Interrupt intents.
type LuaConsoleWidget struct {
	termui.BaseWidget
	term     *termui.CompositeTerminal
	clip     termui.TerminalClipboard
	handlers *ConsoleHandlers

	history    []string
	histIndex  int
	histDraft  string
	promptLive bool
}

// NewLuaConsoleWidget builds an empty Lua console view.
func NewLuaConsoleWidget() *LuaConsoleWidget {
	w := &LuaConsoleWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Lua"},
		term:       termui.NewCompositeTerminalWithPrefix(80, 24, luaScrollback, ""),
	}
	w.wireLocalEcho()
	w.EnsureLivePrompt()
	return w
}

func (m *LuaConsoleWidget) wireLocalEcho() {
	if m == nil || m.term == nil {
		return
	}
	ctl := m.term.Controller()
	if ctl == nil {
		return
	}
	ctl.SetInputHandler(func(b []byte) error {
		return ctl.Write(b)
	})
}

func (m *LuaConsoleWidget) SetClipboard(io termui.ClipboardIO) {
	if m == nil {
		return
	}
	m.clip.Set(io)
	if m.term != nil {
		m.term.SetClipboard(io)
	}
}

func (m *LuaConsoleWidget) SetMouseOrigin(screenX, screenY int) {
	if m != nil && m.term != nil {
		m.term.SetMouseOrigin(screenX, screenY)
	}
}

// WireConsole attaches app handlers to this pane. nil clears handlers.
func (m *LuaConsoleWidget) WireConsole(h *ConsoleHandlers) {
	if m == nil {
		return
	}
	m.handlers = h
}

func (m *LuaConsoleWidget) AppendOutput(line string) {
	if m == nil || line == "" {
		return
	}
	out := termui.TerminalNewlines(line)
	if out == "" {
		return
	}
	if !strings.HasSuffix(out, "\r\n") {
		out += "\r\n"
	}
	ctl := m.term.Controller()
	atBottom := m.term.AtBottom()
	if termui.CurrentLine(ctl) != "" {
		m.term.WriteRaw("\r\n")
	}
	m.term.WriteRaw(out)
	m.promptLive = false
	if atBottom {
		m.term.ScrollToBottom()
	}
}

func (m *LuaConsoleWidget) AppendLines(lines []string) {
	for _, line := range lines {
		m.AppendOutput(line)
	}
}

func (m *LuaConsoleWidget) Clear() {
	if m == nil {
		return
	}
	m.term.Close()
	m.term = termui.NewCompositeTerminalWithPrefix(80, 24, luaScrollback, "")
	m.clip.Apply(m.term)
	m.history = nil
	m.histIndex = 0
	m.histDraft = ""
	m.promptLive = false
	m.wireLocalEcho()
	m.EnsureLivePrompt()
}

func (m *LuaConsoleWidget) InputText() string {
	if m == nil || m.term == nil {
		return ""
	}
	return termui.InputLineText(m.term.Controller())
}

func (m *LuaConsoleWidget) LastHistory() string {
	if m == nil || len(m.history) == 0 {
		return ""
	}
	return m.history[len(m.history)-1]
}

func (m *LuaConsoleWidget) PushHistory(cmd string) {
	if m == nil || cmd == "" {
		return
	}
	if n := len(m.history); n > 0 && m.history[n-1] == cmd {
		m.histIndex = n
		m.histDraft = ""
		return
	}
	m.history = append(m.history, cmd)
	m.histIndex = len(m.history)
	m.histDraft = ""
}

func (m *LuaConsoleWidget) EchoSubmit(cmd string) {
	if m == nil || m.term == nil {
		return
	}
	cur := m.InputText()
	if cur != cmd {
		termui.RewritePromptInput(m.term.Controller(), luaPrompt, cmd)
	}
	m.term.WriteRaw("\r\n")
	m.promptLive = false
}

func (m *LuaConsoleWidget) ApplyCompletion(name string) {
	m.ApplyCompletionFrom(m.InputText(), name)
}

func (m *LuaConsoleWidget) ApplyCompletionFrom(cur, full string) {
	if m == nil || full == "" {
		return
	}
	if full == cur {
		return
	}
	ctl := m.term.Controller()
	if cur != "" && strings.HasPrefix(full, cur) {
		if suffix := full[len(cur):]; suffix != "" {
			_ = ctl.SendString(suffix)
		}
		return
	}
	termui.RewritePromptInput(ctl, luaPrompt, full)
}

func (m *LuaConsoleWidget) InsertInputRune(r rune) {
	m.insertAtCursor(r)
}

func (m *LuaConsoleWidget) BackspaceInput() {
	m.backspaceAtCursor()
}

// The REPL echoes its own input, so there is no readline to turn editing keys
// into cell updates: a \x7f or \x1b[3~ byte written back is parsed as output
// and dropped. Edits therefore rewrite the whole input line themselves.

// insertAtCursor inserts r at the caret. Appending rides the plain echo path;
// inserting mid-line needs a rewrite because xterm overwrites the cell.
func (m *LuaConsoleWidget) insertAtCursor(r rune) {
	if m == nil || m.term == nil {
		return
	}
	ctl := m.term.Controller()
	text, cur := termui.PromptInputState(ctl, luaPrompt)
	if cur >= len(text) {
		_ = ctl.SendInput([]byte(string(r)))
		return
	}
	termui.RewritePromptInput(ctl, luaPrompt, text[:cur]+string(r)+text[cur:])
	termui.MovePromptCursor(ctl, luaPrompt, cur+len(string(r)))
}

// backspaceAtCursor deletes the rune before the caret.
func (m *LuaConsoleWidget) backspaceAtCursor() {
	if m == nil || m.term == nil {
		return
	}
	ctl := m.term.Controller()
	text, cur := termui.PromptInputState(ctl, luaPrompt)
	if cur <= 0 || cur > len(text) {
		return
	}
	_, size := utf8.DecodeLastRuneInString(text[:cur])
	if size == 0 {
		return
	}
	termui.RewritePromptInput(ctl, luaPrompt, text[:cur-size]+text[cur:])
	termui.MovePromptCursor(ctl, luaPrompt, cur-size)
}

// deleteAtCursor deletes the rune under the caret (Delete).
func (m *LuaConsoleWidget) deleteAtCursor() {
	if m == nil || m.term == nil {
		return
	}
	ctl := m.term.Controller()
	text, cur := termui.PromptInputState(ctl, luaPrompt)
	if cur < 0 || cur >= len(text) {
		return
	}
	_, size := utf8.DecodeRuneInString(text[cur:])
	if size == 0 {
		return
	}
	termui.RewritePromptInput(ctl, luaPrompt, text[:cur]+text[cur+size:])
	termui.MovePromptCursor(ctl, luaPrompt, cur)
}

func (m *LuaConsoleWidget) ClearInput() {
	if m == nil || m.term == nil {
		return
	}
	termui.RewritePromptInput(m.term.Controller(), luaPrompt, "")
	m.promptLive = true
}

func (m *LuaConsoleWidget) EnsureLivePrompt() {
	if m == nil || m.term == nil || m.promptLive {
		return
	}
	ctl := m.term.Controller()
	if termui.OnEmptyPromptLine(ctl, luaPrompt) {
		m.promptLive = true
		return
	}
	text := ""
	if termui.OnPromptLine(ctl, luaPrompt) {
		text, _ = termui.PromptInputState(ctl, luaPrompt)
	}
	if termui.CurrentLine(ctl) != "" {
		m.term.WriteRaw("\r\n")
	}
	termui.RewritePromptInput(ctl, luaPrompt, text)
	m.promptLive = true
}

func (m *LuaConsoleWidget) FollowTailAndScroll() {
	if m == nil || m.term == nil {
		return
	}
	if m.term.AtBottom() {
		m.term.ScrollToBottom()
	}
}

func (m *LuaConsoleWidget) ForceFollowTailAndScroll() {
	if m == nil || m.term == nil {
		return
	}
	m.term.ScrollToBottom()
}

func (m *LuaConsoleWidget) SetFocused(focused bool) {
	m.BaseWidget.SetFocused(focused)
}

func (m *LuaConsoleWidget) Draw(c termui.Canvas) {
	if m == nil || m.term == nil {
		return
	}
	m.term.Paint(c, m.Focused())
}

func (m *LuaConsoleWidget) DrawStatusLine(c termui.Canvas, active bool) {
	m.BaseWidget.DrawStatusLine(c, active)
}

func (m *LuaConsoleWidget) HandleEvent(ev tcell.Event) {
	if m == nil {
		return
	}
	switch e := ev.(type) {
	case *tcell.EventMouse:
		if m.term != nil {
			m.term.HandleMouse(e)
		}
	case *tcell.EventClipboard:
		if m.term != nil {
			m.term.PasteBytes(e.Data())
		}
	case *tcell.EventKey:
		m.handleKey(e)
	}
}

func (m *LuaConsoleWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	if m == nil {
		return false
	}
	return m.handleKey(ev)
}

func (m *LuaConsoleWidget) HasTerminalSelection() bool {
	return m != nil && m.term != nil && m.term.HasSelection()
}

func (m *LuaConsoleWidget) ResetTerminalInput() {
	if m != nil && m.term != nil {
		m.term.AfterHostResume()
	}
}

func (m *LuaConsoleWidget) handleKey(ev *tcell.EventKey) bool {
	if m == nil || m.term == nil || ev == nil {
		return false
	}
	if isLuaEnter(ev) {
		if !m.term.AtBottom() {
			m.term.ScrollToBottom()
			return true
		}
		m.submitLine()
		return true
	}
	switch ev.Key() {
	case tcell.KeyUp:
		m.historyPrev()
		return true
	case tcell.KeyDown:
		m.historyNext()
		return true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		m.backspaceAtCursor()
		return true
	case tcell.KeyDelete:
		m.deleteAtCursor()
		return true
	}
	if isLuaCtrlC(ev) {
		if m.term.HasSelection() {
			return m.term.HandleKey(ev)
		}
		if m.handlers != nil && m.handlers.Interrupt != nil {
			m.handlers.Interrupt()
		}
		return true
	}
	if isLuaCtrlD(ev) {
		if m.handlers != nil && m.handlers.EOF != nil {
			m.handlers.EOF()
		}
		return true
	}
	if isLuaCtrl(ev, 'a', tcell.KeyCtrlA) {
		m.cursorHome()
		return true
	}
	if isLuaCtrl(ev, 'e', tcell.KeyCtrlE) {
		m.cursorEnd()
		return true
	}
	if isLuaCtrl(ev, 'u', tcell.KeyCtrlU) {
		m.killToStart()
		return true
	}
	if isLuaCtrl(ev, 'l', tcell.KeyCtrlL) {
		m.refreshScreen()
		return true
	}
	if ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0 {
		m.insertAtCursor(ev.Rune())
		return true
	}
	return m.term.HandleKey(ev)
}

func (m *LuaConsoleWidget) cursorHome() {
	if m == nil || m.term == nil {
		return
	}
	termui.MovePromptCursor(m.term.Controller(), luaPrompt, 0)
}

func (m *LuaConsoleWidget) cursorEnd() {
	if m == nil || m.term == nil {
		return
	}
	ctl := m.term.Controller()
	text, _ := termui.PromptInputState(ctl, luaPrompt)
	termui.MovePromptCursor(ctl, luaPrompt, len(text))
}

func (m *LuaConsoleWidget) killToStart() {
	if m == nil || m.term == nil {
		return
	}
	ctl := m.term.Controller()
	text, cur := termui.PromptInputState(ctl, luaPrompt)
	if cur <= 0 {
		return
	}
	termui.RewritePromptInput(ctl, luaPrompt, text[cur:])
	termui.MovePromptCursor(ctl, luaPrompt, 0)
}

func (m *LuaConsoleWidget) refreshScreen() {
	if m == nil || m.term == nil {
		return
	}
	ctl := m.term.Controller()
	text := ""
	if termui.OnPromptLine(ctl, luaPrompt) {
		text, _ = termui.PromptInputState(ctl, luaPrompt)
	}
	m.term.WriteRaw("\x1b[2J\x1b[H")
	termui.RewritePromptInput(ctl, luaPrompt, text)
	m.promptLive = true
}

func (m *LuaConsoleWidget) submitLine() {
	if m.handlers == nil || m.handlers.Submit == nil {
		return
	}
	ctl := m.term.Controller()
	raw := ""
	if !termui.OnEmptyPromptLine(ctl, luaPrompt) {
		raw = termui.FullInputLineText(ctl, luaPrompt)
	}
	m.handlers.Submit(raw)
}

func (m *LuaConsoleWidget) historyPrev() {
	if len(m.history) == 0 {
		return
	}
	if m.histIndex == len(m.history) {
		m.histDraft = termui.FullInputLineText(m.term.Controller(), luaPrompt)
	}
	if m.histIndex > 0 {
		m.histIndex--
		m.rewriteInput(m.history[m.histIndex])
	}
}

func (m *LuaConsoleWidget) historyNext() {
	if m.histIndex >= len(m.history) {
		return
	}
	m.histIndex++
	text := m.histDraft
	if m.histIndex < len(m.history) {
		text = m.history[m.histIndex]
	}
	m.rewriteInput(text)
}

func (m *LuaConsoleWidget) rewriteInput(text string) {
	if m == nil || m.term == nil {
		return
	}
	termui.RewritePromptInput(m.term.Controller(), luaPrompt, text)
}

func isLuaEnter(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEnter, tcell.KeyCtrlM, tcell.KeyCtrlJ:
		return true
	}
	return false
}

func isLuaCtrlC(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyCtrlC {
		return true
	}
	return ev.Key() == tcell.KeyRune && (ev.Rune() == 'c' || ev.Rune() == 'C') &&
		ev.Modifiers()&tcell.ModCtrl != 0
}

func isLuaCtrlD(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyCtrlD {
		return true
	}
	return ev.Key() == tcell.KeyRune && (ev.Rune() == 'd' || ev.Rune() == 'D') &&
		ev.Modifiers()&tcell.ModCtrl != 0
}

func isLuaCtrl(ev *tcell.EventKey, r rune, key tcell.Key) bool {
	if ev == nil {
		return false
	}
	if ev.Key() == key {
		return true
	}
	if ev.Key() != tcell.KeyRune || ev.Modifiers()&tcell.ModCtrl == 0 {
		return false
	}
	rr := ev.Rune()
	return rr == r || rr == r-'a'+'A'
}
