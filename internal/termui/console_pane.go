package termui

import (
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/platform"
)

// ConsolePane is a natural REPL transcript: scrollback + walking prompt + InputLine.
// It echoes submit as prompt+cmd only (no chat role labels).
type ConsolePane struct {
	BaseWidget

	out   *Viewport
	buf   *platform.Buffer
	input *InputLine

	Prompt      string
	PromptStyle tcell.Style
	TextStyle   tcell.Style
	LineStyle   func(line string) tcell.Style

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
		BaseWidget:  BaseWidget{PaneName: paneName},
		out:         out,
		buf:         buf,
		input:       NewInputLine(),
		PromptStyle: tcell.StyleDefault.Foreground(tcell.ColorYellow),
		TextStyle:   tcell.StyleDefault,
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
	p.out.SetClipboard(io)
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
	p.out.Home()
	p.out.SetFollowTail(true)
	p.input.Clear()
}

// EchoSubmit appends prompt+cmd to scrollback (native REPL echo, not chat labels).
func (p *ConsolePane) EchoSubmit(cmd string) {
	n := p.buf.NumLines()
	promptTrim := strings.TrimSpace(p.Prompt)
	if n > 0 && strings.TrimSpace(p.buf.Line(n-1)) == promptTrim {
		p.buf.SetLine(n-1, p.Prompt+cmd)
		return
	}
	p.buf.AppendLine(p.Prompt + cmd)
}

func (p *ConsolePane) AppendLines(lines []string) {
	for _, line := range lines {
		p.AppendText(line)
	}
}

func (p *ConsolePane) AppendText(text string) {
	if n := p.buf.NumLines(); n > 0 && p.buf.Line(n-1) == p.Prompt {
		p.buf.SetLine(n-1, p.buf.Line(n-1)+text)
		return
	}
	p.buf.AppendLine(text)
}

// StripTrailingBarePrompt removes a trailing empty or bare prompt line from scrollback.
func (p *ConsolePane) StripTrailingBarePrompt() {
	promptTrim := strings.TrimSpace(p.Prompt)
	for p.buf.NumLines() > 0 {
		last := strings.TrimSpace(p.buf.Line(p.buf.NumLines() - 1))
		if last != "" && last != promptTrim {
			return
		}
		p.buf.RemoveLine(p.buf.NumLines() - 1)
		if last == promptTrim {
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
		p.out.HandleEvent(e)

	case *tcell.EventKey:
		if p.HandleBoundKey(e) {
			return
		}
		if isConsoleClipboardKey(e) {
			p.out.HandleEvent(e)
			return
		}
		if e.Key() == tcell.KeyBackspace {
			p.input.Backspace()
			return
		}
		if e.Key() == tcell.KeyRune {
			p.input.InsertRune(e.Rune())
		}
	}
}

func isConsoleClipboardKey(e *tcell.EventKey) bool {
	if e.Key() == tcell.KeyCtrlC || e.Key() == tcell.KeyCtrlX || e.Key() == tcell.KeyCtrlV {
		return true
	}
	if e.Modifiers()&tcell.ModCtrl == 0 || e.Key() != tcell.KeyRune {
		return false
	}
	switch e.Rune() {
	case 'c', 'C', 'x', 'X', 'v', 'V':
		return true
	}
	return false
}

func (p *ConsolePane) Draw(c Canvas) {
	for y := 0; y < c.H(); y++ {
		c.ClearLine(y, tcell.StyleDefault)
	}
	h := c.H()
	if h < 1 {
		return
	}

	n := 0
	if p.buf != nil {
		n = p.buf.NumLines()
	}

	inputY := n
	if !p.out.FollowTail() || inputY > h-1 {
		inputY = h - 1
	}
	if contentH := inputY; contentH > 0 {
		content := c.WithRect(c.ChildRect(0, 0, c.W(), contentH))
		p.out.Draw(content)
	}

	cursorX, under := p.input.Draw(c, 0, inputY, p.Prompt, p.PromptStyle, p.TextStyle)
	p.PaintCursor(c, cursorX, inputY, under)
}
