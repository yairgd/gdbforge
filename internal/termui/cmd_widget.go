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
	case *tcell.EventKey:

		switch e.Key() {

		case tcell.KeyEnter:
			c.history.Add(c.text)
			c.history.ResetNavigation()
			break
		case tcell.KeyUp:
			c.text = c.history.Prev()
			break
		case tcell.KeyDown:
			c.text = c.history.Next()
		}

		return

	}
}

func (m *CmdWidget) Draw(c Canvas) {
	c.Printf(0, 0, tcell.StyleDefault, "test cmd line")
}
