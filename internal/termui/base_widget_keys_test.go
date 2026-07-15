package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/commands"
)

func TestBaseWidgetBindKeyUp(t *testing.T) {
	b := BaseWidget{}
	called := false
	b.BindKey(commands.NewCommand("up", func(args ...any) { called = true }), "<Up>")

	ev := tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	if !b.HandleBoundKey(ev) {
		t.Fatal("expected <Up> handled")
	}
	if !called {
		t.Fatal("action not invoked")
	}
}

func TestBaseWidgetChord(t *testing.T) {
	b := BaseWidget{}
	called := false
	b.BindKey(commands.NewCommand("move", func(args ...any) { called = true }), "<C-w>k")

	if !b.HandleBoundKey(tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone)) {
		t.Fatal("chord start should be handled")
	}
	if !b.HandleBoundKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone)) {
		t.Fatal("chord end should be handled")
	}
	if !called {
		t.Fatal("action not invoked")
	}
}

func TestBaseWidgetUnboundRune(t *testing.T) {
	b := BaseWidget{}
	b.BindKey(commands.NewCommand("up", func(args ...any) {}), "<Up>")
	if b.HandleBoundKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)) {
		t.Fatal("unbound rune must not be handled")
	}
}
