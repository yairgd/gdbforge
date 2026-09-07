package widgets

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/termui"
)

func TestGDBWidgetHomeEndForwardsReadline(t *testing.T) {
	w := NewGDBWidget()
	w.term.WriteRaw("(gdb) hello")

	var sent []byte
	w.term.Controller().SetInputHandler(func(b []byte) error {
		sent = append(sent, b...)
		return nil
	})

	w.HandleFocusKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if string(sent) != "\x01" {
		t.Fatalf("Home: got %q want \\x01", sent)
	}

	sent = nil
	w.HandleFocusKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if string(sent) != "\x05" {
		t.Fatalf("End: got %q want \\x05", sent)
	}
}

func TestGDBWidgetScrollToBottomAfterScrollback(t *testing.T) {
	w := NewGDBWidget()
	for i := 0; i < 30; i++ {
		w.term.WriteRaw("line\r\n")
	}
	w.term.HandleKey(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	if w.term.AtBottom() {
		t.Fatal("expected scrolled up before ScrollToBottom")
	}

	w.ScrollToBottom()
	if !w.term.AtBottom() {
		t.Fatal("ScrollToBottom did not pin viewport to live tail")
	}
}

func TestGDBWidgetDrawSmoke(t *testing.T) {
	w := NewGDBWidget()
	g := termui.NewGrid(40, 10)
	c := termui.NewCanvas(g).WithRect(termui.NewRect(0, 0, 40, 10))
	w.Draw(c)
	w.WriteBoot("(gdb) \n")
	w.AppendLines([]string{">>> AI: ping"})
}
