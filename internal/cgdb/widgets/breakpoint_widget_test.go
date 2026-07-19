package widgets

import (
	"context"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/mcp"
	"github.com/yairgd/cgdb-go/internal/platform"
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
	w := NewBreakpointWidget()
	w.SetPTY(nil, st)
	w.items = []mcp.BreakInfo{
		{Number: 1, Enabled: true, File: "/tmp/a.c", Line: 1},
		{Number: 2, Enabled: false, File: "/tmp/a.c", Line: 2},
	}
	en := w.rowStyle(0, "")
	_, enBg, _ := en.Decompose()
	if enBg != tcell.ColorPurple {
		t.Fatalf("enabled bg=%v want purple", enBg)
	}
	dis := w.rowStyle(1, "")
	_, disBg, _ := dis.Decompose()
	if disBg != tcell.ColorAqua {
		t.Fatalf("disabled bg=%v want aqua", disBg)
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
