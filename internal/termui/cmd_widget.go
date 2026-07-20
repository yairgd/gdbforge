package termui

import (
	"fmt"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/platform"
)

type CmdWidget struct {
	BaseWidget

	history   History
	parser    *commands.CommandParser
	clipboard ClipboardIO
	active    bool
	text      string
	cursor    int
}

func NewCmdWidget(reg *commands.CommandRegistry) *CmdWidget {
	return &CmdWidget{
		BaseWidget: BaseWidget{cursor: NewNativeCursor()},
		history:    NewMemoryHistory(),
		parser:     commands.NewCommandParser(reg),
		active:     false,
	}
}

// SetClipboard wires the shared copy/paste bridge (same as Viewport / ConsolePane).
func (c *CmdWidget) SetClipboard(io ClipboardIO) {
	c.clipboard = io
}

// Active reports whether the cmdline is editing (command / completion mode).
func (c *CmdWidget) Active() bool { return c.active }

// Text returns the current cmdline contents (including leading ':').
func (c *CmdWidget) Text() string { return c.text }

func (c *CmdWidget) emit(ev Event) {
	if c.Events != nil {
		c.Events <- ev
	}
}

func (c *CmdWidget) submitCommand() {
	line := strings.TrimSpace(c.text)
	if strings.HasPrefix(line, ":") {
		line = strings.TrimSpace(line[1:])
	}

	if line == "" {
		c.emit(SubmitMsg{Text: c.text, CmdID: CmdUnknown})
		return
	}

	if err := c.parser.Parse(line); err != nil {
		c.emit(SubmitMsg{Text: c.text, CmdID: CmdUnknown})
		return
	}

	if c.parser.CanExecute() {
		_ = c.parser.Execute()
		return
	}

	c.emit(SubmitMsg{Text: c.text, CmdID: CmdUnknown})
}

func (c *CmdWidget) syncParser() {
	line := c.text
	if strings.HasPrefix(line, ":") {
		line = line[1:]
	}
	c.parser.Sync(line, c.cursor-1)
}

func (c *CmdWidget) tokenBounds() (start, end int) {
	runes := []rune(c.text)
	end = c.cursor
	start = 1
	for i := 1; i < end; i++ {
		if runes[i] == ' ' {
			start = i + 1
		}
	}
	return start, end
}

func (c *CmdWidget) replaceToken(name string) {
	start, end := c.tokenBounds()
	runes := []rune(c.text)
	prefix := string(runes[:start])
	suffix := string(runes[end:])
	c.text = prefix + name + suffix
	c.cursor = len([]rune(prefix + name))
}

// ApplyCompletion replaces the current token with name (wildmenu accept).
func (c *CmdWidget) ApplyCompletion(name string) {
	if name == "" {
		return
	}
	c.replaceToken(name)
}
func (c *CmdWidget) Activate() {
	c.active = true
	c.text = ":"
	c.cursor = 1

}
func (c *CmdWidget) Deativate() {
	c.active = false
	c.text = ""
	c.cursor = 0
	c.history.ResetNavigation()

}

// SetCursorAtLocalX places the caret from a click x relative to the cmdline left edge.
// Column 0 is the leading ':'; the caret never moves onto or before it.
func (c *CmdWidget) SetCursorAtLocalX(localX int) {
	if !c.active {
		return
	}
	n := len([]rune(c.text))
	if n < 1 {
		return
	}
	if localX < 1 {
		localX = 1
	}
	if localX > n {
		localX = n
	}
	c.cursor = localX
}

func (c *CmdWidget) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {

	case *tcell.EventResize:
		return

	case *tcell.EventKey:
		if !c.active {
			return
		}
		if isPasteKey(e) {
			c.pasteAtCursor()
			return
		}
		if isCopyCutKey(e) {
			// No mark selection on cmdline: copy/cut the editable text after ':'.
			c.copyEditable()
			if e.Key() == tcell.KeyCtrlX || (e.Modifiers()&tcell.ModCtrl != 0 && (e.Rune() == 'x' || e.Rune() == 'X')) {
				c.clearEditable()
			}
			return
		}

		switch e.Key() {

		case tcell.KeyEscape:
			c.emit(SubmitMsg{
				Text:  "exit from command mode",
				CmdID: CmdExitMode,
			})
			c.active = false
			c.text = ""
			c.cursor = 0

			return

		case tcell.KeyTAB:
			c.syncParser()
			names := c.parser.SuggestionNames()
			token := c.parser.CurrentToken()
			restArgs := c.parser.CurrentIsRestArgs()

			if c.Ctx.Bus != nil {
				platform.Publish(c.Ctx.Bus, CompletionMsg{
					Input: c.text,
					Token: token,
					Names: names,
				})
			}

			if len(names) != 1 {
				return
			}

			c.replaceToken(names[0])
			if restArgs {
				return
			}
			// Unique tree match: add a trailing space when the command takes more input.
			suggestions := c.parser.Suggestions()
			if len(suggestions) == 1 {
				node := suggestions[0]
				children, _ := node.Complete("")
				if len(children) > 0 || node.RestArgs {
					c.text += " "
					c.cursor = len([]rune(c.text))
				}
			}
			return

		case tcell.KeyEnter:

			c.history.Add(c.text)
			c.history.ResetNavigation()
			c.submitCommand()

			c.active = false
			c.text = ""
			c.cursor = 0

			return

		case tcell.KeyUp:

			c.text = c.history.Prev()

			if c.text != "" && !strings.HasPrefix(c.text, ":") {
				c.text = ":" + c.text
			}

			c.cursor = len([]rune(c.text))
			return

		case tcell.KeyDown:

			c.text = c.history.Next()

			if c.text != "" && !strings.HasPrefix(c.text, ":") {
				c.text = ":" + c.text
			}

			c.cursor = len([]rune(c.text))
			return

		case tcell.KeyLeft:

			if c.cursor > 1 { // keep cursor after ':'
				c.cursor--
			}

			return

		case tcell.KeyRight:

			if c.cursor < len([]rune(c.text)) {
				c.cursor++
			}

			return

		case tcell.KeyCtrlA:

			c.cursor = 1 // after ':'
			return

		case tcell.KeyCtrlE:

			c.cursor = len([]rune(c.text))
			return

		case tcell.KeyBackspace, tcell.KeyBackspace2:

			r := []rune(c.text)

			// deleting ':' exits command mode
			if len(r) == 1 && r[0] == ':' {
				c.emit(SubmitMsg{
					Text:  "exit from command mode",
					CmdID: CmdExitMode,
				})
				c.active = false
				c.text = ""
				c.cursor = 0

				return
			}

			if c.cursor > 1 {

				r = append(r[:c.cursor-1], r[c.cursor:]...)
				c.cursor--

				c.text = string(r)
			}

			return

		default:

			ch := e.Rune()

			if ch == 0 {
				return
			}

			r := []rune(c.text)

			r = append(r, 0)
			copy(r[c.cursor+1:], r[c.cursor:])
			r[c.cursor] = ch

			c.text = string(r)
			c.cursor++

			return
		}
	case *tcell.EventClipboard:
		if !c.active {
			return
		}
		if data := e.Data(); len(data) > 0 {
			c.pasteText(string(data))
		}
	case *tcell.EventMouse:
		if !c.active {
			return
		}
		if isMiddlePaste(e) {
			c.pasteAtCursor()
			return
		}
		if e.Buttons()&tcell.ButtonPrimary != 0 {
			// Caller should prefer SetCursorAtLocalX with a known origin;
			// without a rect, treat absolute X as local (tests / simple hosts).
			x, _ := e.Position()
			c.SetCursorAtLocalX(x)
		}
	case *tcell.EventError:
	case tcell.EventFocus:
	case *tcell.EventInterrupt:
	case *tcell.EventPaste:
	case *tcell.EventTime:
	default:
		panic(fmt.Sprintf("unexpected tcell.Event: %#v", e))
	}
}

func (c *CmdWidget) pasteAtCursor() {
	c.pasteText(c.clipboard.pasteText())
}

func (c *CmdWidget) pasteText(text string) {
	text = firstLinePaste(text)
	if text == "" || !c.active {
		return
	}
	runes := []rune(c.text)
	ins := []rune(text)
	out := make([]rune, 0, len(runes)+len(ins))
	out = append(out, runes[:c.cursor]...)
	out = append(out, ins...)
	out = append(out, runes[c.cursor:]...)
	c.text = string(out)
	c.cursor += len(ins)
}

func (c *CmdWidget) editableText() string {
	runes := []rune(c.text)
	if len(runes) <= 1 {
		return ""
	}
	return string(runes[1:])
}

func (c *CmdWidget) copyEditable() {
	c.clipboard.copyText(c.editableText())
}

func (c *CmdWidget) clearEditable() {
	if !c.active {
		return
	}
	c.text = ":"
	c.cursor = 1
}

func (m *CmdWidget) Draw(c Canvas) {
	c.ClearLine(0, tcell.StyleDefault)

	if !m.active {
		return
	}

	// Left-anchored: never horizontal-scroll the cmdline; clip overflow on the right.
	width := c.W()
	runes := []rune(m.text)
	visible := runes
	if width > 0 && len(runes) > width {
		if width == 1 {
			visible = []rune("…")
		} else {
			visible = append(append([]rune{}, runes[:width-1]...), '…')
		}
	}
	c.Print(0, 0, tcell.StyleDefault, string(visible))

	cur := m.cursor
	if cur < 0 {
		cur = 0
	}
	under := ' '
	if cur < len(runes) {
		under = runes[cur]
	}
	paintX := cur
	if width > 0 && paintX >= width {
		paintX = width - 1
		under = '…'
	}
	m.Cursor().Paint(c, paintX, 0, under)
}

func (m *CmdWidget) DrawStatusLine(c Canvas, active bool) {}
