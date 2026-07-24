package main

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/platform"
)

func TestKeyBindingsFallthroughUp(t *testing.T) {
	a := &DebuggerApp{}
	a.TermApp = nil // unused
	a.keyBindings = commands.NewKeyBindingRegistry()
	a.keyBindings.Bind(
		commands.NewHandledCommand("code-up", func() bool { return false }),
		"<Up>",
	)
	ev := tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	if a.tryKeyBindings(a.keyBindings, ev) {
		t.Fatal("declined Up should fall through")
	}
}

func TestKeyBindingsConsumeNext(t *testing.T) {
	a := &DebuggerApp{}
	called := false
	a.keyBindings = commands.NewKeyBindingRegistry()
	a.keyBindings.Bind(
		commands.NewCommand("gdb-next", func(args ...any) { called = true }),
		"n",
	)
	ev := tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone)
	if !a.tryKeyBindings(a.keyBindings, ev) {
		t.Fatal("n should be consumed")
	}
	if !called {
		t.Fatal("action not run")
	}
}

func TestKeyBindingsPartialChord(t *testing.T) {
	a := &DebuggerApp{}
	a.keyBindings = commands.NewKeyBindingRegistry()
	a.keyBindings.Bind(
		commands.NewCommand("move", func(args ...any) {}),
		"<C-w>h",
	)
	cw := tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone)
	if !a.tryKeyBindings(a.keyBindings, cw) {
		t.Fatal("C-w should start partial chord")
	}
	if !a.keyBindings.InPartial() {
		t.Fatal("expected InPartial after C-w")
	}
}

func TestHandledCommandAPI(t *testing.T) {
	reg := commands.NewKeyBindingRegistry()
	reg.Bind(commands.NewHandledCommand("x", func() bool { return false }), "x")
	key, ok := platform.KeyFromEvent(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if !ok {
		t.Fatal("key")
	}
	completed, handled := reg.HandleKey(key)
	if !handled || completed {
		t.Fatalf("completed=%v handled=%v want false,true", completed, handled)
	}
}
