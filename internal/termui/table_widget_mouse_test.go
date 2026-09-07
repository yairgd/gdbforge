package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

func TestTableWidgetDoubleClickCopiesCell(t *testing.T) {
	ctx := platform.NewAppContext()
	w := NewTableWidget(ctx)
	var copied string
	w.SetClipboard(ClipboardIO{Copy: func(s string) { copied = s }})

	tbl := w.Table()
	tbl.SetShowHeader(false)
	tbl.SetGutter(2)
	tbl.AddColumn("ID")
	tbl.AddColumn("Loc")
	want := "/home/yair/hel.c:8"
	w.SetFill(func(t *Table) {
		t.AddRow("1", want)
	})

	g := NewGrid(40, 3)
	c := NewCanvas(g).WithRect(NewRect(0, 0, 40, 3))
	w.Draw(c)

	x, y := w.screenX+10, w.screenY // middle of path cell
	row, col, _, ok := w.HitCell(x, y)
	if !ok || row != 0 || col != 1 {
		t.Fatalf("HitCell row=%d col=%d ok=%v want 0,1", row, col, ok)
	}
	if w.TryDoubleClickWord(tcell.NewEventMouse(x, y, tcell.ButtonPrimary, 0)) {
		t.Fatal("first click should not consume")
	}
	if !w.TryDoubleClickWord(tcell.NewEventMouse(x, y, tcell.ButtonPrimary, 0)) {
		t.Fatal("second click should consume double-click")
	}
	if copied != want {
		t.Fatalf("copied=%q want %q", copied, want)
	}
	if !w.HasSelection() {
		t.Fatal("expected cell selection highlight")
	}
	if w.SelectedText() != want {
		t.Fatalf("SelectedText=%q", w.SelectedText())
	}
}

func TestTableWidgetHitCell(t *testing.T) {
	ctx := platform.NewAppContext()
	w := NewTableWidget(ctx)
	tbl := w.Table()
	tbl.SetShowHeader(false)
	tbl.SetGutter(2)
	tbl.AddColumn("A")
	tbl.AddColumn("B")
	w.SetFill(func(t *Table) {
		t.AddRow("xx", "alpha beta")
	})

	g := NewGrid(20, 3)
	c := NewCanvas(g).WithRect(NewRect(0, 0, 20, 3))
	w.Draw(c)

	row, col, off, ok := w.HitCell(w.screenX+4, w.screenY)
	if !ok || row != 0 || col != 1 || off != 0 {
		t.Fatalf("HitCell=%d,%d,%d ok=%v", row, col, off, ok)
	}
}
