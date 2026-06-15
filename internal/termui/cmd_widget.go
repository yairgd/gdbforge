package termui

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"
)

type CmdWidget struct {
	history     core.History
	completer   core.AutoCompleter
	active      bool
	completions []string
	compIndex   int
	text        string
	cursor      int
}

func NewCmdWidget() *CmdWidget {

	completer := core.NewSimpleCompleter([]string{
		"break", "continue", "next", "step",
		"print", "bt", "info", "run", "quit",
	})

	return &CmdWidget{
		history:   core.NewMemoryHistory(),
		completer: completer,
		active:    false,
	}
}

// ////////////////////////
// EVENTS
// ////////////////////////
func (c *CmdWidget) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {

	case *tcell.EventResize:
		return

	case *tcell.EventKey:

		switch e.Key() {

		case tcell.KeyTAB:

			matches := c.completer.Complete(c.text)

			if len(matches) == 1 {
				c.text = matches[0]
				c.cursor = len([]rune(c.text))
			}

			return
		case tcell.KeyEsc:

		case tcell.KeyEnter:
			c.history.Add(c.text)
			c.history.ResetNavigation()

			// execute command here

			c.text = ""
			c.cursor = 0
			return

		case tcell.KeyUp:
			c.text = c.history.Prev()
			c.cursor = len([]rune(c.text))
			return

		case tcell.KeyDown:
			c.text = c.history.Next()
			c.cursor = len([]rune(c.text))
			return

		case tcell.KeyLeft:
			if c.cursor > 0 {
				c.cursor--
			}
			return

		case tcell.KeyRight:
			if c.cursor < len([]rune(c.text)) {
				c.cursor++
			}
			return

		case tcell.KeyBackspace, tcell.KeyBackspace2:

			r := []rune(c.text)

			if c.cursor > 0 {
				r = append(r[:c.cursor-1], r[c.cursor:]...)
				c.cursor--
			}

			c.text = string(r)
			return
		case tcell.KeyCtrlA:
			c.cursor = 0
			return

		case tcell.KeyCtrlE:
			c.cursor = len([]rune(c.text))
			return
		default:

			ch := e.Rune()

			if ch != 0 {

				r := []rune(c.text)

				r = append(r, 0)
				copy(r[c.cursor+1:], r[c.cursor:])
				r[c.cursor] = ch

				c.text = string(r)
				c.cursor++
			}

			return
		}
	}
}

func (m *CmdWidget) Draw(c Canvas) {
	c.ClearLine(0, tcell.StyleDefault)
	c.Print(0, 0, tcell.StyleDefault, m.text)
	c.ShowCursor(m.cursor, 0)
}
