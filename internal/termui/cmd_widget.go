package termui

import (
	"fmt"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
)

type CmdWidget struct {
	history     History
	completer   AutoCompleter
	active      bool
	completions []string
	compIndex   int
	text        string
	cursor      int

	Events chan Event
}

func NewCmdWidget(completer AutoCompleter) *CmdWidget {

	if completer == nil {
		completer = NewSimpleCompleter(nil)
	}

	return &CmdWidget{
		history:   NewMemoryHistory(),
		completer: completer,
		active:    false,
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

	parts := strings.Fields(line)
	if len(parts) == 0 {
		c.emit(SubmitMsg{Text: c.text, CmdID: CmdUnknown})
		return
	}

	args := ""
	if len(parts) > 1 {
		args = strings.Join(parts[1:], " ")
	}

	matches := c.completer.Complete(parts[0])
	if len(matches) == 0 {
		c.emit(SubmitMsg{
			Text:  c.text,
			CmdID: CmdUnknown,
			Args:  args,
		})
		return
	}

	c.emit(SubmitMsg{
		Text:  c.text,
		CmdID: matches[0].ID,
		Args:  args,
	})
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

		// Vim-like behavior:
		// command line is inactive until ':' is pressed.
		//		if !c.active {
		//
		//			if e.Key() == tcell.KeyRune && e.Rune() == ':' {
		//				c.active = true
		//				c.text = ":"
		//				c.cursor = 1
		//			}
		//
		//			return
		//		}

		switch e.Key() {

		//		case tcell.KeyEsc:
		//
		//			c.active = false
		//			c.text = ""
		//			c.cursor = 0
		//			c.history.ResetNavigation()
		//
		//			return
		//
		case tcell.KeyTAB:

			prefix := c.text

			if strings.HasPrefix(prefix, ":") {
				prefix = prefix[1:]
			}

			if idx := strings.IndexAny(prefix, " \t"); idx >= 0 {
				prefix = prefix[:idx]
			}

			matches := c.completer.Complete(prefix)

			if len(matches) == 1 {

				c.text = ":" + matches[0].Name
				c.cursor = len([]rune(c.text))
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
		c.HideCursor()
		return
	}

	c.Print(0, 0, tcell.StyleDefault, m.text)
	c.ShowCursor(m.cursor, 0)
}
