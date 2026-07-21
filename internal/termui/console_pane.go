package termui

import (
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

// ConsolePane is a natural REPL transcript: scrollback + walking prompt + InputLine.
// It echoes submit as prompt+cmd only (no chat role labels).
type ConsolePane struct {
	BaseWidget

	out       *Viewport
	buf       *platform.Buffer
	input     *InputLine
	clipboard ClipboardIO

	Prompt      string
	PromptStyle tcell.Style
	TextStyle   tcell.Style
	LineStyle   func(line string) tcell.Style

	// livePrompt means the last buffer line is the active prompt; input is
	// drawn on the same row immediately after that line (GDB/bash/ssh).
	livePrompt bool

	// inputEnabled controls whether keystrokes edit the InputLine (default true).
	// Set false for read-only consoles (e.g. program stdout Output pane).
	inputEnabled bool

	OnSubmit    func(cmd string)
	OnInterrupt func()
	OnEOF       func()
}

func NewConsolePane(paneName string) *ConsolePane {
	buf := platform.NewBuffer()
	out := NewViewport(buf)
	out.SetFollowTail(true)
	out.SetReadOnly(true)
	out.SetCursorVisible(false)

	p := &ConsolePane{
		BaseWidget:   BaseWidget{PaneName: paneName},
		out:          out,
		buf:          buf,
		input:        NewInputLine(),
		PromptStyle:  tcell.StyleDefault.Foreground(tcell.ColorYellow),
		TextStyle:    tcell.StyleDefault,
		inputEnabled: true,
	}
	p.SetCursor(NewNativeCursor())
	out.LineStyle = func(line string) tcell.Style {
		if p.LineStyle != nil {
			return p.LineStyle(line)
		}
		return tcell.StyleDefault
	}
	p.initKeyBindings()
	return p
}

func (p *ConsolePane) Input() *InputLine { return p.input }
func (p *ConsolePane) Viewport() *Viewport { return p.out }
func (p *ConsolePane) Buffer() *platform.Buffer { return p.buf }

func (p *ConsolePane) SetClipboard(io ClipboardIO) {
	p.clipboard = io
	p.out.SetClipboard(io)
}

// SetANSI enables ANSI/SGR color rendering in the scrollback viewport.
func (p *ConsolePane) SetANSI(on bool) {
	p.out.ANSI = on
}

// SetInputEnabled toggles whether the pane accepts typed input.
// When false, Draw uses the full height for scrollback (no input row).
func (p *ConsolePane) SetInputEnabled(on bool) {
	p.inputEnabled = on
	if !on {
		p.livePrompt = false
	}
}

// InputEnabled reports whether typed input is accepted.
func (p *ConsolePane) InputEnabled() bool {
	return p.inputEnabled
}

// SetLivePrompt marks whether the last buffer line hosts the input caret
// (drawn on the same row, after the line's visible end).
func (p *ConsolePane) SetLivePrompt(on bool) {
	if !p.inputEnabled {
		p.livePrompt = false
		return
	}
	p.livePrompt = on
}

// LivePrompt reports whether input is attached to the last buffer line.
func (p *ConsolePane) LivePrompt() bool {
	return p.livePrompt
}

// EnsureLivePrompt makes the last buffer line equal to Prompt and marks it live
// so the input line continues on that same row.
// No-op when Prompt is empty (callers must attach a host line themselves).
func (p *ConsolePane) EnsureLivePrompt() {
	if p.buf == nil || p.Prompt == "" {
		return
	}
	if n := p.buf.NumLines(); n > 0 && p.buf.Line(n-1) == p.Prompt {
		p.livePrompt = true
		return
	}
	p.buf.AppendLine(p.Prompt)
	p.livePrompt = true
}

func (p *ConsolePane) SetFocused(focused bool) {
	p.BaseWidget.SetFocused(focused)
	p.out.SetCursorVisible(false)
}

func (p *ConsolePane) initKeyBindings() {
	p.input.BindKeys(p)
	p.BindKeyFunc("submit", func(args ...any) { p.submit() }, "<Enter>", "<C-m>", "<C-j>")
	p.BindKeyFunc("interrupt", func(args ...any) { p.interrupt() }, "<C-c>")
	p.BindKeyFunc("eof", func(args ...any) { p.eof() }, "<C-d>")
	p.BindKeyFunc("clear", func(args ...any) { p.Clear() }, "<C-l>")
	p.BindKeyFunc("scroll-up", func(args ...any) { p.out.ScrollPageUp(10) }, "<PgUp>")
	p.BindKeyFunc("scroll-down", func(args ...any) { p.out.ScrollPageDown(10) }, "<PgDn>")
}

func (p *ConsolePane) submit() {
	cmd := p.input.Text()
	if p.OnSubmit != nil {
		p.OnSubmit(cmd)
	}
}

func (p *ConsolePane) interrupt() {
	if p.out.HasSelection() {
		p.out.CopySelection()
		return
	}
	if p.OnInterrupt != nil {
		p.OnInterrupt()
	}
}

func (p *ConsolePane) eof() {
	if p.OnEOF != nil {
		p.OnEOF()
	}
}

func (p *ConsolePane) Clear() {
	if p.buf != nil {
		p.buf.Clear()
	}
	p.livePrompt = false
	p.out.Home()
	p.out.SetFollowTail(true)
	p.input.Clear()
}

// EchoSubmit appends prompt+cmd to scrollback (native REPL echo, not chat labels).
func (p *ConsolePane) EchoSubmit(cmd string) {
	n := p.buf.NumLines()
	if p.livePrompt && n > 0 {
		p.buf.SetLine(n-1, p.buf.Line(n-1)+cmd)
		p.livePrompt = false
		return
	}
	promptTrim := strings.TrimSpace(p.Prompt)
	if p.Prompt != "" && n > 0 && strings.TrimSpace(p.buf.Line(n-1)) == promptTrim {
		p.buf.SetLine(n-1, p.Prompt+cmd)
		p.livePrompt = false
		return
	}
	// Empty Prompt (GDB): echo onto a bare (gdb) host line if present.
	if p.Prompt == "" && n > 0 && strings.TrimSpace(p.buf.Line(n-1)) == "(gdb)" {
		host := p.buf.Line(n - 1)
		if !strings.HasSuffix(host, " ") {
			host += " "
		}
		p.buf.SetLine(n-1, host+cmd)
		p.livePrompt = false
		return
	}
	p.buf.AppendLine(p.Prompt + cmd)
	p.livePrompt = false
}

func (p *ConsolePane) AppendLines(lines []string) {
	p.dropLivePromptHost()
	for _, line := range lines {
		p.AppendText(line)
	}
}

func (p *ConsolePane) AppendText(text string) {
	if p.Prompt != "" {
		if n := p.buf.NumLines(); n > 0 && p.buf.Line(n-1) == p.Prompt {
			p.buf.SetLine(n-1, p.buf.Line(n-1)+text)
			return
		}
	}
	p.buf.AppendLine(text)
}

// dropLivePromptHost removes the last line when it is only the live input host,
// so new scrollback is not inserted after a stale prompt.
func (p *ConsolePane) dropLivePromptHost() {
	if !p.livePrompt || p.buf == nil {
		return
	}
	n := p.buf.NumLines()
	if n == 0 {
		p.livePrompt = false
		return
	}
	p.buf.RemoveLine(n - 1)
	p.livePrompt = false
}

// StripTrailingBarePrompt removes a trailing empty or bare prompt line from scrollback.
func (p *ConsolePane) StripTrailingBarePrompt() {
	promptTrim := strings.TrimSpace(p.Prompt)
	for p.buf.NumLines() > 0 {
		last := strings.TrimSpace(p.buf.Line(p.buf.NumLines() - 1))
		bareGdb := last == "(gdb)"
		bareConfigured := promptTrim != "" && last == promptTrim
		if last != "" && !bareGdb && !bareConfigured {
			return
		}
		p.buf.RemoveLine(p.buf.NumLines() - 1)
		p.livePrompt = false
		if bareGdb || bareConfigured {
			return
		}
	}
}

func (p *ConsolePane) FollowTailAndScroll() {
	p.out.SetFollowTail(true)
	p.out.ScrollToBottom()
}

func (p *ConsolePane) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		// Middle-click paste into the input line (Linux terminal convention).
		if p.inputEnabled && isMiddlePaste(e) {
			p.pasteIntoInput()
			return
		}
		p.out.HandleEvent(e)

	case *tcell.EventKey:
		if p.inputEnabled && isPasteKey(e) {
			p.pasteIntoInput()
			return
		}
		if p.HandleBoundKey(e) {
			return
		}
		if isConsoleClipboardKey(e) {
			p.out.HandleEvent(e)
			return
		}
		if !p.inputEnabled {
			return
		}
		if e.Key() == tcell.KeyBackspace {
			p.input.Backspace()
			return
		}
		if e.Key() == tcell.KeyRune {
			p.input.InsertRune(e.Rune())
		}

	case *tcell.EventClipboard:
		if !p.inputEnabled {
			return
		}
		if data := e.Data(); len(data) > 0 {
			p.pasteText(string(data))
		}

	case *tcell.EventPaste:
		// Bracketed-paste start/end markers only; payload is EventClipboard or keys.
	}
}

// pasteIntoInput inserts the system clipboard into the input line (not scrollback).
func (p *ConsolePane) pasteIntoInput() {
	p.pasteText(p.clipboard.pasteText())
}

func (p *ConsolePane) pasteText(text string) {
	p.input.InsertText(firstLinePaste(text))
}

func (p *ConsolePane) Draw(c Canvas) {
	for y := 0; y < c.H(); y++ {
		c.ClearLine(y, tcell.StyleDefault)
	}
	h := c.H()
	if h < 1 {
		return
	}

	if !p.inputEnabled {
		p.out.Draw(c)
		return
	}

	n := 0
	if p.buf != nil {
		n = p.buf.NumLines()
	}

	attach := p.livePrompt && p.out.FollowTail() && n > 0
	inputY := n
	if attach {
		inputY = n - 1
	}
	if !p.out.FollowTail() {
		// Scrolled away from the tail: input alone on the bottom row.
		inputY = h - 1
		attach = false
	} else if inputY > h-1 {
		// Full viewport: keep live host+input on the bottom row (do not
		// clear attach — that painted (gdb) one line above the caret).
		inputY = h - 1
	}
	if contentH := inputY; contentH > 0 {
		content := c.WithRect(c.ChildRect(0, 0, c.W(), contentH))
		if attach {
			p.out.OmitTail = 1
		}
		p.out.Draw(content)
		p.out.OmitTail = 0
	}

	promptCols := 0
	prompt := p.Prompt
	if attach {
		host := p.buf.Line(n - 1)
		hostStyle := tcell.StyleDefault
		if p.LineStyle != nil {
			hostStyle = p.LineStyle(host)
		}
		if p.out.ANSI {
			promptCols = c.DrawANSIText(0, inputY, 0, host, hostStyle, nil)
		} else {
			col := 0
			for _, ch := range host {
				if col >= c.W() {
					break
				}
				c.SetContent(col, inputY, ch, hostStyle)
				col++
			}
			promptCols = col
		}
		prompt = ""
	}

	cursorX, under := p.input.Draw(c, promptCols, inputY, prompt, p.PromptStyle, p.TextStyle)
	p.PaintCursor(c, cursorX, inputY, under)
}
