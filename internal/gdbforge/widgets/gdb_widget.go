package widgets

import (
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/mitext"
	"github.com/yairgd/gdbforge/internal/termui"
)

// GDBWidget is a display-only GDB console view (ConsolePane chrome).
// The app owns GDBClient, MI parsing, and quit/send policy; it paints via
// these methods and handles OnSubmit / OnInterrupt / OnEOF intents.
type GDBWidget struct {
	console          *termui.ConsolePane
	promptStyleToken string // "(gdb)" or "(dlv)" for yellow line styling
}

// NewGDBWidget builds an empty GDB console view. Wire intents with SetOn*.
func NewGDBWidget() *GDBWidget {
	console := termui.NewConsolePane("GDB")
	// No standing Prompt: Draw must not invent "(gdb)" while waiting.
	console.Prompt = ""
	console.PromptStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow)
	w := &GDBWidget{console: console, promptStyleToken: mitext.MIPromptToken}
	console.LineStyle = w.lineStyle
	return w
}

// SetPromptStyleToken sets which prompt token gets yellow styling ("(gdb)" / "(dlv)").
func (m *GDBWidget) SetPromptStyleToken(token string) {
	if m == nil {
		return
	}
	if token == "" {
		token = mitext.MIPromptToken
	}
	m.promptStyleToken = token
}

// SetANSI enables ANSI/SGR color rendering in the console scrollback
// (make/gcc diagnostics, Delve source listings, …).
func (m *GDBWidget) SetANSI(on bool) {
	if m == nil || m.console == nil {
		return
	}
	m.console.SetANSI(on)
}

func (m *GDBWidget) lineStyle(line string) tcell.Style {
	st := tcell.StyleDefault
	if strings.HasPrefix(line, ">>>") {
		return st.Foreground(tcell.ColorTeal).Bold(true)
	}
	// Ctrl-C feedback from GDB log stream (&"Quit" / &"❌️ Quit"), not UI text.
	if mitext.IsCtrlCQuitLog(line) {
		return st.Foreground(tcell.ColorRed).Bold(true)
	}
	tok := m.promptStyleToken
	if tok == "" {
		tok = mitext.MIPromptToken
	}
	if strings.HasPrefix(line, tok) {
		return st.Foreground(tcell.ColorYellow)
	}
	return st
}

// SetOnSubmit registers the Enter handler (app controller).
func (m *GDBWidget) SetOnSubmit(fn func(cmd string)) {
	if m == nil || m.console == nil {
		return
	}
	m.console.OnSubmit = fn
}

// SetOnInterrupt registers the Ctrl-C handler (app controller).
func (m *GDBWidget) SetOnInterrupt(fn func()) {
	if m == nil || m.console == nil {
		return
	}
	m.console.OnInterrupt = fn
}

// SetOnSuspend registers the Ctrl-Z handler (app controller).
func (m *GDBWidget) SetOnSuspend(fn func()) {
	if m == nil || m.console == nil {
		return
	}
	m.console.OnSuspend = fn
}

// SetOnEOF registers the Ctrl-D handler (app controller).
func (m *GDBWidget) SetOnEOF(fn func()) {
	if m == nil || m.console == nil {
		return
	}
	m.console.OnEOF = fn
}

func (m *GDBWidget) SetClipboard(io termui.ClipboardIO) {
	if m == nil || m.console == nil {
		return
	}
	m.console.SetClipboard(io)
}

func (m *GDBWidget) AppendLines(lines []string) {
	if m == nil || m.console == nil || len(lines) == 0 {
		return
	}
	m.console.AppendLines(lines)
	m.console.FollowTailAndScroll()
}

// AppendTargetText paints raw inferior stdout (legacy gdbtargetprint mode).
func (m *GDBWidget) AppendTargetText(text string) {
	if m == nil || m.console == nil || text == "" {
		return
	}
	m.console.AppendText(text)
	m.console.FollowTailAndScroll()
}

func (m *GDBWidget) Clear() {
	if m == nil || m.console == nil {
		return
	}
	m.console.Clear()
}

// InputText returns the current input-line contents.
func (m *GDBWidget) InputText() string {
	if m == nil || m.console == nil || m.console.Input() == nil {
		return ""
	}
	return m.console.Input().Text()
}

// LastHistory returns the previous submitted command (empty Enter repeat).
func (m *GDBWidget) LastHistory() string {
	if m == nil || m.console == nil || m.console.Input() == nil {
		return ""
	}
	return m.console.Input().LastHistory()
}

// PushHistory records a submitted command in the input history.
func (m *GDBWidget) PushHistory(cmd string) {
	if m == nil || m.console == nil || m.console.Input() == nil || cmd == "" {
		return
	}
	m.console.Input().PushHistory(cmd)
}

// EchoSubmit paints prompt+cmd into scrollback (native REPL echo).
func (m *GDBWidget) EchoSubmit(cmd string) {
	if m == nil || m.console == nil {
		return
	}
	m.console.EchoSubmit(cmd)
}

// ClearInput clears the editable input line.
func (m *GDBWidget) ClearInput() {
	if m == nil || m.console == nil || m.console.Input() == nil {
		return
	}
	m.console.Input().Clear()
}

// ApplyCompletion replaces the input line with name.
func (m *GDBWidget) ApplyCompletion(name string) {
	if m == nil || m.console == nil || m.console.Input() == nil || name == "" {
		return
	}
	m.console.Input().SetText(name)
	m.console.FollowTailAndScroll()
}

// InsertInputRune types into the input line (e.g. while wildmenu is open).
func (m *GDBWidget) InsertInputRune(r rune) {
	if m == nil || m.console == nil || m.console.Input() == nil {
		return
	}
	m.console.Input().InsertRune(r)
	m.console.FollowTailAndScroll()
}

// BackspaceInput deletes one character from the input line.
func (m *GDBWidget) BackspaceInput() {
	if m == nil || m.console == nil || m.console.Input() == nil {
		return
	}
	m.console.Input().Backspace()
	m.console.FollowTailAndScroll()
}

func (m *GDBWidget) SetFocused(focused bool) {
	if m == nil || m.console == nil {
		return
	}
	m.console.SetFocused(focused)
}

func (m *GDBWidget) Draw(c termui.Canvas) {
	if m == nil || m.console == nil {
		return
	}
	m.console.Draw(c)
}

func (m *GDBWidget) Viewport() *termui.Viewport {
	if m == nil || m.console == nil {
		return nil
	}
	return m.console.Viewport()
}

func (m *GDBWidget) DrawStatusLine(c termui.Canvas, active bool) {
	if m == nil || m.console == nil {
		return
	}
	m.console.DrawStatusLine(c, active)
}

func (m *GDBWidget) FollowTailAndScroll() {
	if m == nil || m.console == nil {
		return
	}
	m.console.FollowTailAndScroll()
}

func (m *GDBWidget) LivePrompt() bool {
	if m == nil || m.console == nil {
		return false
	}
	return m.console.LivePrompt()
}

func (m *GDBWidget) SetLivePrompt(on bool) {
	if m == nil || m.console == nil {
		return
	}
	m.console.SetLivePrompt(on)
}

// BeginLiveHost appends scrollback lines then a live host line for typing.
// Text comes from the controller / gdb session (e.g. QuitConfirmLines); the
// view does not invent GDB CLI wording.
func (m *GDBWidget) BeginLiveHost(scrollback []string, host string) {
	if m == nil || m.console == nil {
		return
	}
	if len(scrollback) > 0 {
		m.console.AppendLines(scrollback)
	}
	if host != "" {
		if buf := m.console.Buffer(); buf != nil {
			buf.AppendLine(host)
		}
		m.console.SetLivePrompt(true)
	}
	m.ClearInput()
	m.console.FollowTailAndScroll()
}

// AttachGdbPrompt paints GDB's MI prompt record as the live input host.
func (m *GDBWidget) AttachGdbPrompt(fromGDB string) {
	if m == nil || m.console == nil {
		return
	}
	host := mitext.LivePromptHost(fromGDB)
	if host == "" {
		return
	}
	buf := m.console.Buffer()
	if buf == nil {
		return
	}
	n := buf.NumLines()
	if n > 0 {
		last := buf.Line(n - 1)
		bare := mitext.IsBareMIPromptHost(last) || strings.TrimSpace(last) == strings.TrimSpace(fromGDB)
		if bare {
			buf.SetLine(n-1, host)
			m.console.SetLivePrompt(true)
			return
		}
	}
	buf.AppendLine(host)
	m.console.SetLivePrompt(true)
}

// StripTrailingGdbPrompt drops a trailing bare MI/Delve prompt host before new output.
func (m *GDBWidget) StripTrailingGdbPrompt() {
	if m == nil || m.console == nil {
		return
	}
	buf := m.console.Buffer()
	if buf == nil {
		return
	}
	tok := m.promptStyleToken
	if tok == "" {
		tok = mitext.MIPromptToken
	}
	for buf.NumLines() > 0 {
		last := buf.Line(buf.NumLines() - 1)
		bare := mitext.IsBareMIPromptHost(last) || strings.TrimSpace(last) == tok
		if strings.TrimSpace(last) != "" && !bare {
			return
		}
		buf.RemoveLine(buf.NumLines() - 1)
		m.console.SetLivePrompt(false)
		if bare {
			return
		}
	}
}

// PaintMiDisplay applies DisplayLines / Ctrl-C quit logs from an MI update.
// confirming suppresses attaching a new (gdb) host (quit prompt owns the line).
func (m *GDBWidget) PaintMiDisplay(upd MiPaintUpdate, confirming, includeTarget bool) {
	if m == nil || m.console == nil {
		return
	}
	lines := upd.DisplayLines
	if includeTarget {
		lines = append(lines, upd.TargetLines...)
	}
	var rest []string
	painted := false
	for _, line := range lines {
		if mitext.IsCtrlCQuitLog(line) {
			m.console.EchoSubmit(line)
			painted = true
			continue
		}
		rest = append(rest, line)
	}
	if len(rest) > 0 {
		m.console.AppendLines(rest)
		m.StripTrailingGdbPrompt()
		painted = true
	}
	if upd.PromptReady && !confirming {
		m.AttachGdbPrompt(upd.PromptLine)
	}
	if painted || (upd.PromptReady && !confirming) {
		m.console.FollowTailAndScroll()
	}
}

// PaintDlvDisplay paints Delve CLI output (peer of PaintMiDisplay).
// confirming suppresses attaching a new (dlv) host (yes/no prompt owns the line).
func (m *GDBWidget) PaintDlvDisplay(displayLines []string, promptReady bool, promptLine string, confirming bool) {
	if m == nil || m.console == nil {
		return
	}
	painted := false
	if len(displayLines) > 0 {
		m.console.AppendLines(displayLines)
		m.StripTrailingGdbPrompt()
		painted = true
	}
	if promptReady && !confirming {
		m.AttachGdbPrompt(promptLine)
	}
	if painted || (promptReady && !confirming) {
		m.console.FollowTailAndScroll()
	}
}

func (m *GDBWidget) HandleEvent(ev tcell.Event) {
	if m == nil || m.console == nil {
		return
	}
	m.console.HandleEvent(ev)
}
