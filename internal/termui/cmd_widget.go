package termui

import (
	"fmt"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/commands"
	"github.com/yairgd/cgdb-go/internal/platform"
)

type CmdWidget struct {
	BaseWidget

	history History
	parser  *commands.CommandParser
	active  bool
	text    string
	cursor  int
}

func NewCmdWidget(reg *commands.CommandRegistry) *CmdWidget {
	return &CmdWidget{
		BaseWidget: BaseWidget{cursor: NewNativeCursor()},
		history:    NewMemoryHistory(),
		parser:     commands.NewCommandParser(reg),
		active:     false,
	}
}

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

func (c *CmdWidget) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {

	case *tcell.EventResize:
		return

	case *tcell.EventKey:

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
	case *tcell.EventError:
	case tcell.EventFocus:
	case *tcell.EventInterrupt:
	case *tcell.EventMouse:
	case *tcell.EventPaste:
	case *tcell.EventTime:
	default:
		panic(fmt.Sprintf("unexpected tcell.Event: %#v", e))
	}
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
