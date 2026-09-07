package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

type terminalMouseStub struct {
	BaseWidget
	term *CompositeTerminal
}

func (s *terminalMouseStub) SetMouseOrigin(x, y int) { s.term.SetMouseOrigin(x, y) }
func (s *terminalMouseStub) HandleEvent(ev tcell.Event) {
	if me, ok := ev.(*tcell.EventMouse); ok {
		s.term.HandleMouse(me)
	}
}
func (s *terminalMouseStub) Draw(c Canvas) { s.term.Paint(c, true) }

// Release over a sibling pane must still finish selection on the focused terminal.
func TestWidgetTreeMouseReleaseOnFocusedTerminal(t *testing.T) {
	left := &terminalMouseStub{
		BaseWidget: BaseWidget{PaneName: "GDB"},
		term:       NewCompositeTerminal(20, 5, 100),
	}
	right := &stubPane{id: "code"}
	tree := NewWidgetTree(left)
	tree.Split(Vertical, right)
	tree.BuildLayout(NewCanvas(NewGrid(40, 6)).WithRect(NewRect(0, 0, 40, 6)))

	var copied string
	left.term.SetClipboard(ClipboardIO{Copy: func(s string) { copied = s }})
	_ = left.term.ctl.WriteString("hello world\r\n")

	// Select in left (focused) pane.
	tree.HandleEvent(tcell.NewEventMouse(0, 0, tcell.ButtonPrimary, 0))
	tree.HandleEvent(tcell.NewEventMouse(5, 0, tcell.ButtonPrimary, 0))
	// Release over the right pane (screen x >= 20).
	tree.HandleEvent(tcell.NewEventMouse(25, 0, tcell.ButtonNone, 0))

	if copied != "hello" {
		t.Fatalf("copied %q want %q", copied, "hello")
	}
}
