package widgets

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
)

func TestBreakpointWidgetEmptyMessage(t *testing.T) {
	w := NewBreakpointWidget()
	lines := w.LinesForTest()
	if len(lines) != 1 || lines[0] != "no breakpoints" {
		t.Fatalf("empty=%v", lines)
	}
}

func TestBreakpointWidgetSetItemsAndToggleIntent(t *testing.T) {
	w := NewBreakpointWidget()
	w.SetFocused(true)
	w.SetItems([]mcp.BreakInfo{
		{Number: 1, Enabled: true, File: "/tmp/a.c", Line: 10},
	})
	if lines := w.LinesForTest(); len(lines) != 1 || lines[0] != "  1  y  /tmp/a.c:10" {
		t.Fatalf("display=%q", lines)
	}
	var gotIdx int = -1
	w.OnToggle = func(i int) { gotIdx = i }
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone)) {
		t.Fatal("e")
	}
	if gotIdx != 0 {
		t.Fatalf("toggle idx=%d", gotIdx)
	}
}

func TestBreakpointWidgetDeleteIntent(t *testing.T) {
	w := NewBreakpointWidget()
	w.SetItems([]mcp.BreakInfo{
		{Number: 3, Enabled: true, File: "/tmp/a.c", Line: 5},
	})
	var gotIdx int = -1
	w.OnDelete = func(i int) { gotIdx = i }
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone)) {
		t.Fatal("d")
	}
	if gotIdx != 0 {
		t.Fatalf("delete idx=%d", gotIdx)
	}
}

func TestBreakpointWidgetBreakColorsFromState(t *testing.T) {
	st := platform.NewAppState()
	st.SetBreakColor(tcell.ColorPurple)
	st.SetBreakDisabledColor(tcell.ColorAqua)
	st.SetMarkColor(tcell.ColorNavy)
	st.SetMarkDimColor(tcell.ColorSilver)
	w := NewBreakpointWidget()
	w.SetAppState(st)
	w.SetItems([]mcp.BreakInfo{
		{Number: 1, Enabled: true, File: "/tmp/a.c", Line: 1},
		{Number: 2, Enabled: false, File: "/tmp/a.c", Line: 2},
	})
	w.SelectIndex(0)

	sel := w.rowStyle(0, "")
	_, selBg, _ := sel.Decompose()
	if selBg != tcell.ColorSilver {
		t.Fatalf("unfocused selected bg=%v want silver", selBg)
	}
	w.SetFocused(true)
	sel = w.rowStyle(0, "")
	_, selBg, _ = sel.Decompose()
	if selBg != tcell.ColorNavy {
		t.Fatalf("focused selected bg=%v want navy", selBg)
	}

	dis := w.rowStyle(1, "")
	_, disBg, _ := dis.Decompose()
	if disBg != tcell.ColorAqua {
		t.Fatalf("disabled bg=%v want aqua", disBg)
	}
}

func TestBreakpointWidgetActivateOnMove(t *testing.T) {
	w := NewBreakpointWidget()
	w.SetFocused(true)
	w.SetItems([]mcp.BreakInfo{
		{Number: 1, Enabled: true, File: "/tmp/a.c", Line: 10},
		{Number: 2, Enabled: true, File: "/tmp/b.c", Line: 20},
	})
	var got mcp.BreakInfo
	w.OnActivate = func(bp mcp.BreakInfo) { got = bp }
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) {
		t.Fatal("down")
	}
	if got.Number != 2 || got.Line != 20 {
		t.Fatalf("activated=%v", got)
	}
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)) {
		t.Fatal("enter")
	}
	if got.Number != 2 {
		t.Fatalf("enter activated=%v", got)
	}
}

func TestBreakpointWidgetWheelActivates(t *testing.T) {
	w := NewBreakpointWidget()
	w.SetFocused(true)
	w.SetItems([]mcp.BreakInfo{
		{Number: 1, Enabled: true, File: "/tmp/a.c", Line: 10},
		{Number: 2, Enabled: true, File: "/tmp/b.c", Line: 20},
	})
	var got mcp.BreakInfo
	w.OnActivate = func(bp mcp.BreakInfo) { got = bp }
	w.HandleEvent(tcell.NewEventMouse(0, 0, tcell.WheelDown, 0))
	if w.Selected() != 1 || got.Number != 2 {
		t.Fatalf("wheel down selected=%d activated=%v", w.Selected(), got)
	}
}
