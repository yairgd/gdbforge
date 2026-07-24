package widgets

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

func TestThreadWidgetSetItems(t *testing.T) {
	w := NewThreadWidget()
	if got := w.LinesForTest(); len(got) != 1 || got[0] != "no threads" {
		t.Fatalf("empty=%v", got)
	}
	w.SetItems([]models.ThreadInfo{
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
	w.SetItems([]models.StackFrame{
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
	w.SetItems([]models.ThreadInfo{
		{ID: "1", State: "stopped", Current: true},
		{ID: "2", State: "running"},
	})
	w.selected = 1
	var got models.ThreadInfo
	w.OnActivate = func(th models.ThreadInfo) { got = th }
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
	w.SetItems([]models.StackFrame{
		{Level: 0, Func: "main", File: "a.c", Line: 1},
		{Level: 1, Func: "foo", File: "b.c", Line: 2},
	})
	w.selected = 1
	var got models.StackFrame
	w.OnActivate = func(fr models.StackFrame) { got = fr }
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
	w.SetItems([]models.StackFrame{
		{Level: 0, Func: "main", File: "a.c", Line: 1},
		{Level: 1, Func: "foo", File: "b.c", Line: 2},
	})
	var got models.StackFrame
	w.OnActivate = func(fr models.StackFrame) { got = fr }
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
	w.SetItems([]models.ThreadInfo{
		{ID: "1", State: "stopped", Current: true},
		{ID: "2", State: "running"},
	})
	var got models.ThreadInfo
	w.OnActivate = func(th models.ThreadInfo) { got = th }
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) {
		t.Fatal("down")
	}
	if got.ID != "2" {
		t.Fatalf("activated=%v", got)
	}
}

func TestCallStackWidgetWheelActivates(t *testing.T) {
	w := NewCallStackWidget()
	w.SetFocused(true)
	w.SetItems([]models.StackFrame{
		{Level: 0, Func: "main"},
		{Level: 1, Func: "foo"},
	})
	var got models.StackFrame
	w.OnActivate = func(fr models.StackFrame) { got = fr }
	w.HandleEvent(tcell.NewEventMouse(0, 0, tcell.WheelDown, 0))
	if w.Selected() != 1 || got.Func != "foo" {
		t.Fatalf("wheel down selected=%d activated=%v", w.Selected(), got)
	}
	w.HandleEvent(tcell.NewEventMouse(0, 0, tcell.WheelUp, 0))
	if w.Selected() != 0 || got.Func != "main" {
		t.Fatalf("wheel up selected=%d activated=%v", w.Selected(), got)
	}
}

func TestCallStackWidgetSelectedFrame(t *testing.T) {
	w := NewCallStackWidget()
	if _, ok := w.SelectedFrame(); ok {
		t.Fatal("empty list should have no frame")
	}
	w.SetItems([]models.StackFrame{
		{Level: 0, Func: "main", File: "/tmp/a.c", Line: 10},
		{Level: 1, Func: "foo", File: "/tmp/b.c", Line: 20},
	})
	fr, ok := w.SelectedFrame()
	if !ok || fr.Func != "main" || fr.Line != 10 {
		t.Fatalf("selected=%v ok=%v", fr, ok)
	}
	w.move(1)
	fr, ok = w.SelectedFrame()
	if !ok || fr.Func != "foo" || fr.Line != 20 {
		t.Fatalf("after move selected=%v ok=%v", fr, ok)
	}
}

func TestCallStackWidgetProgramPointStyle(t *testing.T) {
	st := debugstate.New(platform.NewAppState())
	st.SetMarkColor(tcell.ColorNavy)
	st.SetMarkDimColor(tcell.ColorSilver)
	st.SetStopLocation("/tmp/a.c", 10)
	// Browsed location differs (mouse picked another frame) — green must stay on #0.
	st.SetCurrentLocation("/tmp/b.c", 20)

	w := NewCallStackWidget()
	w.SetAppState(st)
	w.SetItems([]models.StackFrame{
		{Level: 0, Func: "main", File: "a.c", Line: 10},
		{Level: 1, Func: "foo", File: "/tmp/b.c", Line: 20},
	})
	w.move(1)

	br := w.rowStyle(0, "")
	_, brBg, _ := br.Decompose()
	if brBg != platform.DefaultStackBreakColor {
		t.Fatalf("frame0+stopPC bg=%v want %v", brBg, platform.DefaultStackBreakColor)
	}
	f1 := w.rowStyle(1, "")
	_, f1Bg, _ := f1.Decompose()
	if f1Bg == platform.DefaultStackBreakColor {
		t.Fatal("frame 1 must not use stack break green")
	}
}

func TestThreadWidgetProgramPointStyle(t *testing.T) {
	st := debugstate.New(platform.NewAppState())
	st.SetMarkColor(tcell.ColorNavy)
	st.SetMarkDimColor(tcell.ColorSilver)
	st.SetCurrentLocation("/tmp/a.c", 10)
	st.SetStopLocation("/tmp/a.c", 10)

	w := NewThreadWidget()
	w.SetAppState(st)
	w.SetItems([]models.ThreadInfo{
		{ID: "1", State: "stopped", File: "a.c", Line: 10, Current: true},
		{ID: "2", State: "stopped", File: "a.c", Line: 10, Current: false},
	})
	w.move(1)

	br := w.rowStyle(0, "")
	_, brBg, _ := br.Decompose()
	if brBg != platform.DefaultStackBreakColor {
		t.Fatalf("current+PC bg=%v want %v", brBg, platform.DefaultStackBreakColor)
	}
	other := w.rowStyle(1, "")
	_, otherBg, _ := other.Decompose()
	if otherBg == platform.DefaultStackBreakColor {
		t.Fatal("non-current thread must not be green")
	}
}

func TestThreadWidgetWheelActivates(t *testing.T) {
	w := NewThreadWidget()
	w.SetFocused(true)
	w.SetItems([]models.ThreadInfo{
		{ID: "1", State: "stopped", Current: true},
		{ID: "2", State: "running"},
	})
	var got models.ThreadInfo
	w.OnActivate = func(th models.ThreadInfo) { got = th }
	w.HandleEvent(tcell.NewEventMouse(0, 0, tcell.WheelDown, 0))
	if w.Selected() != 1 || got.ID != "2" {
		t.Fatalf("wheel down selected=%d activated=%v", w.Selected(), got)
	}
	w.HandleEvent(tcell.NewEventMouse(0, 0, tcell.WheelUp, 0))
	if w.Selected() != 0 || got.ID != "1" {
		t.Fatalf("wheel up selected=%d activated=%v", w.Selected(), got)
	}
}

func TestListWidgetsMouseSyncSelection(t *testing.T) {
	bp := NewBreakpointWidget()
	bp.SetFocused(true)
	bp.SetItems([]models.BreakInfo{
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
	th.SetItems([]models.ThreadInfo{
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
	cs.SetItems([]models.StackFrame{
		{Level: 0, Func: "main"},
		{Level: 1, Func: "start"},
	})
	cs.viewport.CursorLine = 1
	cs.syncSelectedFromViewport()
	if cs.selected != 1 {
		t.Fatalf("callstack selected=%d", cs.selected)
	}
}

func TestCallStackDragDoesNotActivateUntilRelease(t *testing.T) {
	w := NewCallStackWidget()
	w.SetFocused(true)
	w.SetItems([]models.StackFrame{
		{Level: 0, Func: "main"},
		{Level: 1, Func: "start"},
	})
	var n int
	w.OnActivate = func(fr models.StackFrame) { n++ }

	// Press + drag motion samples must not activate.
	w.HandleEvent(tcell.NewEventMouse(0, 0, tcell.ButtonPrimary, 0))
	w.HandleEvent(tcell.NewEventMouse(1, 0, tcell.ButtonPrimary, 0))
	w.HandleEvent(tcell.NewEventMouse(2, 0, tcell.ButtonPrimary, 0))
	if n != 0 {
		t.Fatalf("activate during drag: %d", n)
	}
	// Release without a text selection → one activate.
	w.HandleEvent(tcell.NewEventMouse(2, 0, tcell.ButtonNone, 0))
	if n != 1 {
		t.Fatalf("activate on release: %d want 1", n)
	}
}

func TestThreadWidgetHorizontalScrollKeys(t *testing.T) {
	w := NewThreadWidget()
	w.SetFocused(true)
	w.SetItems([]models.ThreadInfo{
		{ID: "thread-with-a-very-long-identifier-0001", State: "stopped-waiting", File: "/tmp/a.c", Line: 99, Current: true},
		{ID: "2", State: "running", File: "b.c", Line: 2},
	})
	// Establish pane width so ViewScrollColRight can advance Left.
	g := termui.NewGrid(8, 4)
	w.Draw(termui.NewCanvas(g).WithRect(termui.NewRect(0, 0, 8, 4)))

	sel := w.Selected()
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)) {
		t.Fatal("Right not handled")
	}
	if w.ViewportLeftForTest() != 1 {
		t.Fatalf("Left=%d want 1 (line=%q)", w.ViewportLeftForTest(), w.LinesForTest()[0])
	}
	if w.Selected() != sel {
		t.Fatalf("selection changed from %d to %d", sel, w.Selected())
	}
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)) {
		t.Fatal("Left not handled")
	}
	if w.ViewportLeftForTest() != 0 {
		t.Fatalf("Left=%d want 0", w.ViewportLeftForTest())
	}
	// Up/Down must not wipe horizontal offset.
	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)) {
		t.Fatal("Right")
	}
	_ = w.HandleFocusKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if w.ViewportLeftForTest() != 1 {
		t.Fatalf("Left reset on move: %d", w.ViewportLeftForTest())
	}
}
