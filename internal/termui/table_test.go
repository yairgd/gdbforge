package termui

import (
	"strings"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

func TestTableLayoutColumnWidths(t *testing.T) {
	tbl := NewTable()
	tbl.SetShowHeader(true)
	tbl.AddColumn("ID")
	tbl.AddColumn("Name")
	tbl.AddRow("1", "alice")
	tbl.AddRow("22", "bob-long")

	lay := tbl.Layout()
	if len(lay.colWidths) != 2 {
		t.Fatalf("colWidths len=%d", len(lay.colWidths))
	}
	if lay.colWidths[0] != 2 {
		t.Fatalf("col0 width=%d want 2", lay.colWidths[0])
	}
	if lay.colWidths[1] != 8 {
		t.Fatalf("col1 width=%d want 8", lay.colWidths[1])
	}
	if lay.stickyRows != 1 {
		t.Fatalf("stickyRows=%d want 1", lay.stickyRows)
	}
}

func TestTableTruncateInPaint(t *testing.T) {
	tbl := NewTable()
	tbl.SetShowHeader(false)
	tbl.AddColumnWidth("X", 4)
	tbl.AddRow("hello-world")

	buf := NewCellBuffer(4, 1)
	rv := NewRectViewport()
	tbl.PaintVisibleDefault(buf, rv, 4, 1)

	cell, _ := buf.Get(3, 0)
	if cell.Rune != '…' {
		t.Fatalf("last rune=%q want ellipsis", cell.Rune)
	}
}

func TestTableStickyHeaderAndVerticalPan(t *testing.T) {
	tbl := NewTable()
	tbl.SetShowHeader(true)
	tbl.AddColumn("A")
	for i := 0; i < 5; i++ {
		tbl.AddRow(string(rune('a' + i)))
	}

	buf := NewCellBuffer(3, 3) // 1 header + 2 data rows visible
	rv := NewRectViewport()
	rv.SetOrigin(0, 2)

	tbl.PaintVisibleDefault(buf, rv, 3, 3)

	// row 0 = header
	h0, _ := buf.Get(0, 0)
	if h0.Rune != 'A' {
		t.Fatalf("header=%q want A", h0.Rune)
	}
	// row 1 = data row 2 ('c')
	d0, _ := buf.Get(0, 1)
	if d0.Rune != 'c' {
		t.Fatalf("data row0=%q want c", d0.Rune)
	}
	d1, _ := buf.Get(0, 2)
	if d1.Rune != 'd' {
		t.Fatalf("data row1=%q want d", d1.Rune)
	}
}

func TestTableHorizontalPan(t *testing.T) {
	tbl := NewTable()
	tbl.SetShowHeader(false)
	tbl.AddColumnWidth("Left", 4)
	tbl.AddColumnWidth("Right", 4)
	tbl.SetGutter(1)
	tbl.AddRow("aaaa", "bbbb")

	buf := NewCellBuffer(5, 1)
	rv := NewRectViewport()
	rv.SetOrigin(5, 0) // align window start with second column

	tbl.PaintVisibleDefault(buf, rv, 5, 1)

	var b strings.Builder
	for x := 0; x < 5; x++ {
		c, _ := buf.Get(x, 0)
		b.WriteRune(c.Rune)
	}
	got := strings.TrimSpace(b.String())
	if got != "bbbb" {
		t.Fatalf("panned line=%q want bbbb", got)
	}
}

func TestTableWidgetDrawAndPanKeys(t *testing.T) {
	ctx := platform.NewAppContext()
	w := NewTableWidget(ctx)
	w.InitPanKeyBindings()
	w.PaneName = "Demo"
	tbl := w.Table()
	tbl.SetShowHeader(true)
	tbl.AddColumn("C1")
	tbl.AddColumn("C2")
	for i := 0; i < 10; i++ {
		tbl.AddRow("r"+string(rune('0'+i)), "x")
	}

	g := NewGrid(6, 4)
	c := NewCanvas(g).WithRect(NewRect(0, 0, 6, 4))
	w.Draw(c)

	ev := tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	if !w.HandleFocusKey(ev) {
		t.Fatal("Down should be consumed")
	}
	w.Draw(c)

	if w.rv.Origin.Y != 1 {
		t.Fatalf("Origin.Y=%d want 1 after Down", w.rv.Origin.Y)
	}

	evLeft := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
	w.HandleFocusKey(evLeft)
	if w.rv.Origin.X != 0 {
		t.Fatalf("Origin.X=%d want 0 with narrow content", w.rv.Origin.X)
	}
}

func TestTableWidgetSetFill(t *testing.T) {
	ctx := platform.NewAppContext()
	w := NewTableWidget(ctx)
	tbl := w.Table()
	tbl.SetShowHeader(false)
	tbl.AddColumn("V")
	n := 0
	w.SetFill(func(t *Table) {
		n++
		t.AddRow("filled")
	})

	g := NewGrid(4, 2)
	c := NewCanvas(g).WithRect(NewRect(0, 0, 4, 2))
	w.Draw(c)
	w.Draw(c)
	if n != 2 {
		t.Fatalf("fill calls=%d want 2", n)
	}
	if tbl.NumRows() != 1 {
		t.Fatalf("rows=%d want 1", tbl.NumRows())
	}
}

func TestTableContentOverflows(t *testing.T) {
	tbl := NewTable()
	tbl.SetShowHeader(true)
	tbl.AddColumn("WideColumnName")
	tbl.AddRow("data")
	if !tbl.ContentOverflows(4, 3) {
		t.Fatal("expected horizontal overflow")
	}
}
