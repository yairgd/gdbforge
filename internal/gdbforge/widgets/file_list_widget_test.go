package widgets

import (
	"testing"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

func TestFileListWidgetSetItems(t *testing.T) {
	w := NewFileListWidget()
	if got := w.LinesForTest(); len(got) != 1 || got[0] != "no files" {
		t.Fatalf("empty=%v", got)
	}
	w.SetItems([]string{"/tmp/a.c", "/proj/b.c"})
	lines := w.LinesForTest()
	if len(lines) != 2 || lines[0] != "1  /tmp/a.c" || lines[1] != "2  /proj/b.c" {
		t.Fatalf("lines=%v", lines)
	}
}

func TestFileListWidgetOpenEnter(t *testing.T) {
	ctx := testWidgetCtx()
	var opened string
	platform.Subscribe(ctx.Bus, func(msg events.OpenSourceMsg) { opened = msg.Path })

	w := NewFileListWidget()
	w.Ctx = ctx
	w.SetFocused(true)
	w.SetItems([]string{"/tmp/a.c", "/tmp/b.c"})
	w.move(1)
	w.openSelected()
	if opened != "/tmp/b.c" {
		t.Fatalf("opened=%q", opened)
	}
}

func TestFileListWidgetMouseSelectThenOpen(t *testing.T) {
	ctx := testWidgetCtx()
	var opened string
	platform.Subscribe(ctx.Bus, func(msg events.OpenSourceMsg) { opened = msg.Path })

	w := NewFileListWidget()
	w.Ctx = ctx
	w.SetFocused(true)
	w.SetItems([]string{"/tmp/a.c", "/tmp/b.c"})

	g := termui.NewGrid(40, 4)
	w.Draw(termui.NewCanvas(g).WithRect(termui.NewRect(0, 0, 40, 4)))

	// First click on row 1: select only.
	w.HandleEvent(tcell.NewEventMouse(0, 1, tcell.ButtonPrimary, 0))
	if w.Selected() != 1 {
		t.Fatalf("selected=%d", w.Selected())
	}
	if opened != "" {
		t.Fatalf("first click must not open, got %q", opened)
	}

	// Second click on same row after double-click timeout: open (not word copy).
	time.Sleep(450 * time.Millisecond)
	w.HandleEvent(tcell.NewEventMouse(0, 1, tcell.ButtonPrimary, 0))
	if opened != "/tmp/b.c" {
		t.Fatalf("opened=%q", opened)
	}
}

func TestFileListWidgetMarkColorFromState(t *testing.T) {
	st := debugstate.New(platform.NewAppState())
	st.SetMarkColor(tcell.ColorNavy)
	w := NewFileListWidget()
	w.SetAppState(st)
	if w.markColor() != tcell.ColorNavy {
		t.Fatalf("mark=%v", w.markColor())
	}
}
