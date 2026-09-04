package widgets

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/termui"
)

func typeLua(t *testing.T, w *LuaConsoleWidget, s string) {
	t.Helper()
	for _, r := range s {
		w.HandleFocusKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
}

func luaLine(w *LuaConsoleWidget) (string, int) {
	return termui.PromptInputState(w.term.Controller(), luaPrompt)
}

func TestLuaConsoleBackspaceDeletesChar(t *testing.T) {
	w := NewLuaConsoleWidget()
	typeLua(t, w, "print")
	if got, _ := luaLine(w); got != "print" {
		t.Fatalf("typed %q want print", got)
	}

	w.HandleFocusKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	got, cur := luaLine(w)
	if got != "prin" || cur != 4 {
		t.Fatalf("after backspace text=%q cursor=%d want prin/4", got, cur)
	}

	// Ctrl-H is delivered as KeyBackspace on some terminals.
	w.HandleFocusKey(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	if got, cur = luaLine(w); got != "pri" || cur != 3 {
		t.Fatalf("after ctrl-h text=%q cursor=%d want pri/3", got, cur)
	}
}

func TestLuaConsoleBackspaceStopsAtPrompt(t *testing.T) {
	w := NewLuaConsoleWidget()
	typeLua(t, w, "ab")
	for i := 0; i < 5; i++ {
		w.HandleFocusKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	}
	got, cur := luaLine(w)
	if got != "" || cur != 0 {
		t.Fatalf("text=%q cursor=%d want empty/0", got, cur)
	}
	if !termui.OnEmptyPromptLine(w.term.Controller(), luaPrompt) {
		t.Fatal("prompt was eaten by backspace")
	}
}

func TestLuaConsoleBackspaceMidLine(t *testing.T) {
	w := NewLuaConsoleWidget()
	typeLua(t, w, "abcd")
	w.cursorHome()
	termui.MovePromptCursor(w.term.Controller(), luaPrompt, 2)

	w.HandleFocusKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	got, cur := luaLine(w)
	if got != "acd" || cur != 1 {
		t.Fatalf("text=%q cursor=%d want acd/1", got, cur)
	}
}

func TestLuaConsoleDeleteKeyRemovesCharUnderCursor(t *testing.T) {
	w := NewLuaConsoleWidget()
	typeLua(t, w, "abcd")
	termui.MovePromptCursor(w.term.Controller(), luaPrompt, 1)

	w.HandleFocusKey(tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone))
	got, cur := luaLine(w)
	if got != "acd" || cur != 1 {
		t.Fatalf("text=%q cursor=%d want acd/1", got, cur)
	}

	w.cursorEnd()
	w.HandleFocusKey(tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone))
	if got, _ = luaLine(w); got != "acd" {
		t.Fatalf("delete at end changed text: %q", got)
	}
}

// Typing mid-line must shift the tail, not overwrite it: xterm has no insert
// mode, so a plain echo would replace the cell under the caret.
func TestLuaConsoleInsertMidLineShiftsTail(t *testing.T) {
	w := NewLuaConsoleWidget()
	typeLua(t, w, "ac")
	termui.MovePromptCursor(w.term.Controller(), luaPrompt, 1)

	typeLua(t, w, "b")
	got, cur := luaLine(w)
	if got != "abc" || cur != 2 {
		t.Fatalf("text=%q cursor=%d want abc/2", got, cur)
	}
}

func TestLuaConsoleBackspaceThenRetype(t *testing.T) {
	w := NewLuaConsoleWidget()
	typeLua(t, w, "prinf")
	w.HandleFocusKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	typeLua(t, w, "t(1)")
	if got, _ := luaLine(w); got != "print(1)" {
		t.Fatalf("text=%q want print(1)", got)
	}
}
