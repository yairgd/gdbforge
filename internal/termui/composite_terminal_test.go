package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestCompositeTerminalHandleKeyTrie(t *testing.T) {
	c := NewCompositeTerminal(10, 5, 100)
	var got []byte
	c.ctl.SetInputHandler(func(b []byte) error {
		got = append(got, b...)
		return nil
	})

	c.HandleKey(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone))
	if string(got) != "\x03" {
		t.Fatalf("ctrl-c: got %q", got)
	}

	got = nil
	c.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))
	if string(got) != "a" {
		t.Fatalf("rune: got %q", got)
	}

	got = nil
	c.HandleKey(tcell.NewEventKey(tcell.KeyEnter, '\r', tcell.ModNone))
	if string(got) != "\r" {
		t.Fatalf("enter: got %q", got)
	}
}

func TestCompositeTerminalPaintCursor(t *testing.T) {
	c := NewCompositeTerminal(10, 3, 100)
	_ = c.ctl.WriteString("ab")

	cx, cy := c.ctl.Cursor()
	if cx != 2 || cy != 0 {
		t.Fatalf("cursor %d,%d want 2,0", cx, cy)
	}

	g := NewGrid(10, 3)
	cv := Canvas{rect: Rect{w: 10, h: 3}, grid: g}
	c.Paint(cv, true)
	c.Paint(cv, false)
}
