package widgets

import (
	"context"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbx/internal/core"
	"github.com/yairgd/gdbx/internal/mcp"
	"github.com/yairgd/gdbx/internal/platform"
)

func TestBreakpointWidgetEmptyMessage(t *testing.T) {
	w := NewBreakpointWidget()
	lines := w.LinesForTest()
	if len(lines) != 1 || lines[0] != "no breakpoints" {
		t.Fatalf("empty=%v", lines)
	}
}

func TestBreakpointWidgetToggleRemovesFromGDBKeepsRow(t *testing.T) {
	w := NewBreakpointWidget()
	sent := make(chan string, 4)
	w.sess = &bpFakeSess{sent: sent}
	w.SetFocused(true)
	w.MergeFromGDB([]mcp.BreakInfo{
		{Number: 1, Enabled: true, File: "/tmp/a.c", Line: 10},
	})
	if len(w.EnabledBreakInfos()) != 1 {
		t.Fatal("expected one enabled")
	}

	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone)) {
		t.Fatal("e")
	}
	if got := <-sent; got != "-break-delete 1" {
		t.Fatalf("cmd=%q", got)
	}
	if len(w.Items()) != 1 {
		t.Fatalf("row should remain: %v", w.Items())
	}
	if w.Items()[0].Enabled {
		t.Fatal("should be disabled in list")
	}
	if len(w.EnabledBreakInfos()) != 0 {
		t.Fatal("enabled list should be empty")
	}

	// Merge from empty GDB must keep disabled row.
	w.MergeFromGDB(nil)
	if len(w.Items()) != 1 || w.Items()[0].Enabled {
		t.Fatalf("disabled row lost: %v", w.Items())
	}

	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone)) {
		t.Fatal("e re-enable")
	}
	if got := <-sent; got != "break a.c:10" {
		t.Fatalf("reinsert cmd=%q", got)
	}
	if !w.Items()[0].Enabled {
		t.Fatal("should be enabled again")
	}
}

func TestBreakpointWidgetToggleAtFileLine(t *testing.T) {
	w := NewBreakpointWidget()
	sent := make(chan string, 4)
	w.sess = &bpFakeSess{sent: sent}
	w.MergeFromGDB([]mcp.BreakInfo{
		{Number: 2, Enabled: true, File: "/tmp/a.c", Line: 10},
	})
	if !w.ToggleAtFileLine("/tmp/a.c", 10, false) {
		t.Fatal("toggle")
	}
	if got := <-sent; got != "-break-delete 2" {
		t.Fatalf("cmd=%q", got)
	}
	if w.Items()[0].Enabled {
		t.Fatal("disabled")
	}
	if w.ToggleAtFileLine("/tmp/a.c", 99, false) {
		t.Fatal("expected no-op")
	}
}

func TestBreakpointWidgetDeleteRemovesFromList(t *testing.T) {
	w := NewBreakpointWidget()
	sent := make(chan string, 2)
	w.sess = &bpFakeSess{sent: sent}
	w.MergeFromGDB([]mcp.BreakInfo{
		{Number: 3, Enabled: true, File: "/tmp/a.c", Line: 5},
	})
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone)) {
		t.Fatal("d")
	}
	if got := <-sent; got != "-break-delete 3" {
		t.Fatalf("cmd=%q", got)
	}
	if len(w.Items()) != 0 {
		t.Fatalf("list=%v", w.Items())
	}
}

func TestBreakpointWidgetMergeAddsExternal(t *testing.T) {
	w := NewBreakpointWidget()
	w.MergeFromGDB([]mcp.BreakInfo{
		{Number: 1, Enabled: true, File: "/tmp/a.c", Line: 1},
	})
	w.MergeFromGDB([]mcp.BreakInfo{
		{Number: 1, Enabled: true, File: "/tmp/a.c", Line: 1},
		{Number: 2, Enabled: true, File: "/tmp/a.c", Line: 2},
	})
	if len(w.Items()) != 2 {
		t.Fatalf("items=%v", w.Items())
	}
}

func TestBreakpointWidgetBreakColorsFromState(t *testing.T) {
	st := platform.NewAppState()
	st.SetBreakColor(tcell.ColorPurple)
	st.SetBreakDisabledColor(tcell.ColorAqua)
	st.SetMarkColor(tcell.ColorNavy)
	st.SetMarkDimColor(tcell.ColorSilver)
	w := NewBreakpointWidget()
	w.SetPTY(nil, st)
	w.items = []mcp.BreakInfo{
		{Number: 1, Enabled: true, File: "/tmp/a.c", Line: 1},
		{Number: 2, Enabled: false, File: "/tmp/a.c", Line: 2},
	}
	w.selected = 0

	// Unfocused selection uses markdimcolor, not breakcolor.
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
	w.MergeFromGDB([]mcp.BreakInfo{
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

type bpFakeSess struct {
	sent chan string
}

func (f *bpFakeSess) Send(cmd string) error { f.sent <- cmd; return nil }
func (f *bpFakeSess) SendRaw(string) error  { return nil }
func (f *bpFakeSess) Close()                {}
func (f *bpFakeSess) Subscribe() (<-chan core.PtyOutputMsg, func()) {
	ch := make(chan core.PtyOutputMsg)
	return ch, func() {}
}
func (f *bpFakeSess) WithWrite(_ context.Context, fn func(w core.PTYWriter) error) error {
	return fn(bpFakePW{f})
}

type bpFakePW struct{ f *bpFakeSess }

func (p bpFakePW) Send(cmd string) error    { p.f.sent <- cmd; return nil }
func (p bpFakePW) SendRaw(raw string) error { return nil }
