package widgets

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/termui"
)

func TestOutputWidgetHostLine(t *testing.T) {
	w := NewOutputWidget()
	w.AppendHostLine("hello")
	w.AppendHostLine("world")
}

func TestOutputWidgetClear(t *testing.T) {
	w := NewOutputWidget()
	w.AppendHostLine("x")
	w.Clear()
	w.AppendHostLine("y")
}

func TestOutputWidgetClearKeepsClipboard(t *testing.T) {
	w := NewOutputWidget()
	var copied string
	w.SetClipboard(termui.ClipboardIO{Copy: func(s string) { copied = s }})
	_ = w.term.Controller().WriteString("hello world\r\n")

	w.Clear()
	_ = w.term.Controller().WriteString("after clear\r\n")

	w.term.HandleMouse(tcell.NewEventMouse(0, 0, tcell.ButtonPrimary, 0))
	w.term.HandleMouse(tcell.NewEventMouse(5, 0, tcell.ButtonPrimary, 0))
	w.term.HandleMouse(tcell.NewEventMouse(5, 0, tcell.ButtonNone, 0))

	if copied != "after" {
		t.Fatalf("copied %q want %q", copied, "after")
	}
}
