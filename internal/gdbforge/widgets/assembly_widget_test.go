package widgets

import (
	"strings"
	"testing"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

func TestAssemblyWidgetDualCursors(t *testing.T) {
	w := NewAssemblyWidget(nil)
	items := []models.AsmLine{
		{Addr: "0x1000", Inst: "push"},
		{Addr: "0x1001", Inst: "mov"},
		{Addr: "0x1002", Inst: "call"},
		{Addr: "0x1003", Inst: "ret"},
	}
	w.SetItems(items, "0x1001", "0x1001")
	if w.PCAddr() != "0x1001" {
		t.Fatalf("pc=%q", w.PCAddr())
	}
	if w.SelAddr() != "0x1001" {
		t.Fatalf("sel=%q", w.SelAddr())
	}
	w.MoveSel(1)
	if w.SelAddr() != "0x1002" {
		t.Fatalf("after down sel=%q", w.SelAddr())
	}
	if w.PCAddr() != "0x1001" {
		t.Fatalf("pc moved: %q", w.PCAddr())
	}
	lines := w.buf.NumLines()
	if lines != 4 {
		t.Fatalf("lines=%d", lines)
	}
	pcLine := w.buf.Line(1)
	if !strings.Contains(pcLine, asmPCMarker) {
		t.Fatalf("pc marker missing on line1: %q", pcLine)
	}
	other := w.buf.Line(2)
	if strings.Contains(other, asmPCMarker) {
		t.Fatalf("pc marker on browse line: %q", other)
	}
}

func TestAssemblyWidgetSyncSelFromViewport(t *testing.T) {
	w := NewAssemblyWidget(nil)
	items := []models.AsmLine{
		{Addr: "0x1000", Inst: "a"},
		{Addr: "0x1001", Inst: "b"},
		{Addr: "0x1002", Inst: "c"},
	}
	w.SetItems(items, "0x1000", "0x1000")
	w.viewport.CursorLine = 2
	w.syncSelFromViewport()
	if w.SelAddr() != "0x1002" {
		t.Fatalf("sel=%q want 0x1002", w.SelAddr())
	}
	if w.PCAddr() != "0x1000" {
		t.Fatalf("pc moved: %q", w.PCAddr())
	}
}
