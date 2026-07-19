package widgets

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/mcp"
)

func TestThreadWidgetSetItems(t *testing.T) {
	w := NewThreadWidget()
	if got := w.LinesForTest(); len(got) != 1 || got[0] != "no threads" {
		t.Fatalf("empty=%v", got)
	}
	w.SetItems([]mcp.ThreadInfo{
		{ID: "1", State: "stopped", File: "/tmp/a.c", Line: 10, Current: true},
		{ID: "2", State: "running", File: "b.c", Line: 2},
	})
	lines := w.LinesForTest()
	if len(lines) != 2 || lines[0] != "1  stopped  a.c:10" {
		t.Fatalf("lines=%v", lines)
	}
}

func TestCallStackWidgetSetItems(t *testing.T) {
	w := NewCallStackWidget()
	if got := w.LinesForTest(); len(got) != 1 || got[0] != "no frames" {
		t.Fatalf("empty=%v", got)
	}
	w.SetItems([]mcp.StackFrame{
		{Level: 0, Func: "main", File: "/tmp/hello.c", Line: 12},
		{Level: 1, Func: "start", File: "crt.c", Line: 3},
	})
	lines := w.LinesForTest()
	if len(lines) != 2 || lines[0] != "0  main  hello.c:12" {
		t.Fatalf("lines=%v", lines)
	}
}

func TestThreadWidgetActivateEnter(t *testing.T) {
	w := NewThreadWidget()
	w.SetFocused(true)
	w.SetItems([]mcp.ThreadInfo{
		{ID: "1", State: "stopped", Current: true},
		{ID: "2", State: "running"},
	})
	w.selected = 1
	var got mcp.ThreadInfo
	w.OnActivate = func(th mcp.ThreadInfo) { got = th }
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)) {
		t.Fatal("enter")
	}
	if got.ID != "2" {
		t.Fatalf("activated=%v", got)
	}
}

func TestCallStackWidgetActivateEnter(t *testing.T) {
	w := NewCallStackWidget()
	w.SetFocused(true)
	w.SetItems([]mcp.StackFrame{
		{Level: 0, Func: "main", File: "a.c", Line: 1},
		{Level: 1, Func: "foo", File: "b.c", Line: 2},
	})
	w.selected = 1
	var got mcp.StackFrame
	w.OnActivate = func(fr mcp.StackFrame) { got = fr }
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)) {
		t.Fatal("enter")
	}
	if got.Level != 1 || got.Func != "foo" {
		t.Fatalf("activated=%v", got)
	}
}

func TestCallStackWidgetActivateOnMove(t *testing.T) {
	w := NewCallStackWidget()
	w.SetFocused(true)
	w.SetItems([]mcp.StackFrame{
		{Level: 0, Func: "main", File: "a.c", Line: 1},
		{Level: 1, Func: "foo", File: "b.c", Line: 2},
	})
	var got mcp.StackFrame
	w.OnActivate = func(fr mcp.StackFrame) { got = fr }
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) {
		t.Fatal("down")
	}
	if got.Level != 1 || got.Func != "foo" {
		t.Fatalf("activated=%v", got)
	}
}

func TestThreadWidgetActivateOnMove(t *testing.T) {
	w := NewThreadWidget()
	w.SetFocused(true)
	w.SetItems([]mcp.ThreadInfo{
		{ID: "1", State: "stopped", Current: true},
		{ID: "2", State: "running"},
	})
	var got mcp.ThreadInfo
	w.OnActivate = func(th mcp.ThreadInfo) { got = th }
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) {
		t.Fatal("down")
	}
	if got.ID != "2" {
		t.Fatalf("activated=%v", got)
	}
}

func TestListWidgetsMouseSyncSelection(t *testing.T) {
	bp := NewBreakpointWidget()
	bp.SetFocused(true)
	bp.MergeFromGDB([]mcp.BreakInfo{
		{Number: 1, Enabled: true, File: "a.c", Line: 1},
		{Number: 2, Enabled: true, File: "a.c", Line: 2},
	})
	bp.viewport.CursorLine = 1
	bp.syncSelectedFromViewport()
	if bp.Selected() != 1 {
		t.Fatalf("bp selected=%d", bp.Selected())
	}

	th := NewThreadWidget()
	th.SetFocused(true)
	th.SetItems([]mcp.ThreadInfo{
		{ID: "1", State: "stopped"},
		{ID: "2", State: "running"},
	})
	th.viewport.CursorLine = 1
	th.syncSelectedFromViewport()
	if th.selected != 1 {
		t.Fatalf("thread selected=%d", th.selected)
	}

	cs := NewCallStackWidget()
	cs.SetFocused(true)
	cs.SetItems([]mcp.StackFrame{
		{Level: 0, Func: "main"},
		{Level: 1, Func: "start"},
	})
	cs.viewport.CursorLine = 1
	cs.syncSelectedFromViewport()
	if cs.selected != 1 {
		t.Fatalf("callstack selected=%d", cs.selected)
	}
}
