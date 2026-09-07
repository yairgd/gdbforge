package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

func TestTableWidgetSelectionPreservesHorizontalPan(t *testing.T) {
	ctx := platform.NewAppContext()
	w := NewTableWidget(ctx)
	tbl := w.Table()
	tbl.SetShowHeader(false)
	tbl.AddColumnWidth("Line", 0)
	w.SetFill(func(t *Table) {
		t.AddRow("thread-with-a-very-long-identifier-0001")
		t.AddRow("short")
	})

	g := NewGrid(8, 4)
	c := NewCanvas(g).WithRect(NewRect(0, 0, 8, 4))
	w.Draw(c)

	w.BindKeyFunc("scroll-right", func(args ...any) { w.PanRight() }, "<Right>")
	w.BindKeyFunc("down", func(args ...any) { w.MoveSelection(1) }, "<Down>")

	if !w.HandleFocusKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)) {
		t.Fatal("Right")
	}
	if w.RectViewport().Origin.X != 1 {
		t.Fatalf("Origin.X=%d want 1", w.RectViewport().Origin.X)
	}
	w.HandleFocusKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if w.RectViewport().Origin.X != 1 {
		t.Fatalf("Origin.X reset on Down: %d", w.RectViewport().Origin.X)
	}
	if w.SelectedRow() != 1 {
		t.Fatalf("selected=%d want 1", w.SelectedRow())
	}
}

func TestTableWidgetSearchJump(t *testing.T) {
	ctx := platform.NewAppContext()
	w := NewTableWidget(ctx)
	tbl := w.Table()
	tbl.SetShowHeader(false)
	tbl.AddColumn("V")
	w.SetFill(func(t *Table) {
		t.AddRow("alpha")
		t.AddRow("beta")
		t.AddRow("gamma")
	})
	w.SetSearchColor(tcell.ColorYellow)

	g := NewGrid(6, 3)
	c := NewCanvas(g).WithRect(NewRect(0, 0, 6, 3))
	w.Draw(c)

	w.CommitSearch("gamma")
	if w.SelectedRow() != 2 {
		t.Fatalf("selected=%d want 2", w.SelectedRow())
	}
	if !w.CursorInSearchMatch() {
		t.Fatal("cursor should be on match")
	}
}

func TestRebuildTableSearchHits(t *testing.T) {
	tbl := NewTable()
	tbl.SetShowHeader(false)
	tbl.AddColumn("T")
	tbl.AddRow("hello loopy")
	hits := RebuildTableSearch(tbl, "lo")
	if len(hits) != 2 {
		t.Fatalf("hits=%d want 2", len(hits))
	}
}
