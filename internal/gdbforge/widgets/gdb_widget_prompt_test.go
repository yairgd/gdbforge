package widgets

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/termforge"
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

func TestGDBWidgetInputTextStripsPrompts(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"(gdb) info b", "info b"},
		{"(dlv) b main.main", "b main.main"},
		{"(gdb)", ""},
		{"> print x", "print x"},
	}
	for _, tc := range cases {
		w := NewGDBWidget()
		w.term.WriteRaw(tc.raw)
		if got := w.InputText(); got != tc.want {
			t.Errorf("%q: got %q want %q", tc.raw, got, tc.want)
		}
	}
}

// The continuation prompt is stripped from the input line but is not a console
// prompt, so Home/End must still navigate scrollback rather than edit the line.
func TestGDBWidgetHomeNavigatesOnContinuationPrompt(t *testing.T) {
	w := NewGDBWidget()
	for i := 0; i < 30; i++ {
		w.term.WriteRaw("line\r\n")
	}
	w.term.WriteRaw("> print x")

	var sent []byte
	w.term.Controller().SetInputHandler(func(b []byte) error {
		sent = append(sent, b...)
		return nil
	})

	w.HandleFocusKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if len(sent) != 0 {
		t.Fatalf("Home forwarded readline on a continuation prompt: %q", sent)
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
	g := termforge.NewGrid(40, 10)
	c := termforge.NewCanvas(g).WithRect(termforge.NewRect(0, 0, 40, 10))
	w.Draw(c)
	w.WriteBoot("(gdb) \n")
	w.AppendLines([]string{">>> AI: ping"})
}
