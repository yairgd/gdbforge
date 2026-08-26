package widgets

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

func TestAssemblyWidgetDualCursors(t *testing.T) {
	w := NewAssemblyWidget()
	items := []models.AsmLine{
		{Addr: "0x1000", Inst: "push"},
		{Addr: "0x1001", Inst: "mov"},
		{Addr: "0x1002", Inst: "call"},
		{Addr: "0x1003", Inst: "ret"},
	}
	w.SetItems(items, "0x1001", "0x1001", "")
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
	w := NewAssemblyWidget()
	items := []models.AsmLine{
		{Addr: "0x1000", Inst: "a"},
		{Addr: "0x1001", Inst: "b"},
		{Addr: "0x1002", Inst: "c"},
	}
	w.SetItems(items, "0x1000", "0x1000", "")
	w.viewport.CursorLine = 2
	w.syncSelFromViewport()
	if w.SelAddr() != "0x1002" {
		t.Fatalf("sel=%q want 0x1002", w.SelAddr())
	}
	if w.PCAddr() != "0x1000" {
		t.Fatalf("pc moved: %q", w.PCAddr())
	}
}

func TestAssemblyWidgetCGDBView(t *testing.T) {
	w := NewAssemblyWidget()
	w.SetContext([]string{
		"#8  0x5555555551bc in main () at hello.c:12",
		"12      printf(\"%s\\n\",b);",
		"#0  0x7ffff7e5d03a in ?? () from /usr/lib64/libc.so.6",
		"#2  0x7ffff7ec56ea in write () from /usr/lib64/libc.so.6",
	})
	w.SetItems([]models.AsmLine{
		{Addr: "0x7ffff7ec56d0", Inst: "endbr64", Func: "write", Offset: "0"},
		{Addr: "0x7ffff7ec56ea", Inst: "add    $0x18,%rsp", Func: "write", Offset: "26"},
	}, "0x7ffff7ec56ea", "0x7ffff7ec56ea", "write")

	if w.buf.NumLines() < 7 {
		t.Fatalf("lines=%d", w.buf.NumLines())
	}
	if !strings.Contains(w.buf.Line(0), "#8  ") {
		t.Fatalf("missing stack context: %q", w.buf.Line(0))
	}
	if !strings.Contains(w.buf.Line(4), "Dump of assembler code for function write:") {
		t.Fatalf("missing dump header: %q", w.buf.Line(4))
	}
	pcLine := w.buf.Line(6)
	if !strings.Contains(pcLine, asmPCMarker) || !strings.Contains(pcLine, "<+26>:") {
		t.Fatalf("pc/offset line=%q", pcLine)
	}
	// Offsets are right-aligned: <+ 0>: lines up with <+26>:.
	zeroLine := w.buf.Line(5)
	if !strings.Contains(zeroLine, "<+ 0>:") || !strings.Contains(pcLine, "<+26>:") {
		t.Fatalf("offset padding: zero=%q pc=%q", zeroLine, pcLine)
	}
	for i := 0; i < w.buf.NumLines(); i++ {
		if strings.Contains(w.buf.Line(i), "\x1b") {
			t.Fatalf("buffer line %d has ANSI escape: %q", i, w.buf.Line(i))
		}
	}
	if w.SelAddr() != "0x7ffff7ec56ea" {
		t.Fatalf("sel=%q", w.SelAddr())
	}
}

func TestAssemblyWidgetStatusLabelNamed(t *testing.T) {
	w := NewAssemblyWidget()
	w.SetContext([]string{"#2  0x7ffff7ec56ea in write () from /usr/lib64/libc.so.6"})
	w.SetItems([]models.AsmLine{
		{Addr: "0x7ffff7ec56d0", Inst: "endbr64", Offset: "0"},
		{Addr: "0x7ffff7ec56ea", Inst: "add    $0x18,%rsp", Offset: "26"},
		{Addr: "0x7ffff7ec56ee", Inst: "ret", Offset: "30"},
	}, "0x7ffff7ec56ea", "0x7ffff7ec56ea", "write")
	got := w.StatusLabel()
	want := "** #2  0x7ffff7ec56ea in write () from /usr/lib64/libc.so.6 (7ffff7ec56d0 - 7ffff7ec56ee) **"
	if got != want {
		t.Fatalf("status=\n%q\nwant\n%q", got, want)
	}
}

func TestAssemblyWidgetStatusLabelPC(t *testing.T) {
	w := NewAssemblyWidget()
	w.SetItems([]models.AsmLine{
		{Addr: "0x7ffff7e5d03a", Inst: "add    $0x18,%rsp"},
		{Addr: "0x7ffff7e5d03e", Inst: "ret"},
		{Addr: "0x7ffff7e5d1e7", Inst: "nop"},
	}, "0x7ffff7e5d03a", "0x7ffff7e5d03a", "")
	got := w.StatusLabel()
	if !strings.HasPrefix(got, "**    0x7ffff7e5d03a:") || !strings.Contains(got, "add    $0x18,%rsp") {
		t.Fatalf("status=%q", got)
	}
	if !strings.Contains(got, "(7ffff7e5d03a - 7ffff7e5d1e7)") || !strings.HasSuffix(got, " **") {
		t.Fatalf("range missing: %q", got)
	}
}

func TestAssemblyWidgetScrollPastPC(t *testing.T) {
	w := NewAssemblyWidget()
	w.SetContext([]string{"#3  0x7ffff7e5899d in _IO_file_write () from /usr/lib64/libc.so.6"})
	items := make([]models.AsmLine, 40)
	for i := range items {
		items[i] = models.AsmLine{
			Addr:   fmt.Sprintf("0x%x", 0x7ffff7e58970+i*4),
			Inst:   "nop",
			Offset: fmt.Sprintf("%d", i*4),
		}
	}
	// PC near top so first page shows frame header (Top=0; default page height 20).
	w.SetItems(items, items[2].Addr, items[2].Addr, "_IO_file_write")
	if w.viewport.Top != 0 {
		t.Fatalf("Top=%d want 0 so frame header is visible initially", w.viewport.Top)
	}
	for i := 0; i < 25; i++ {
		w.MoveSel(1)
	}
	if w.SelAddr() == items[2].Addr {
		t.Fatal("Down should leave $pc browse line")
	}
	// Frame/Dump header rolls off; scroll up would bring it back.
	if w.viewport.Top < 1 {
		t.Fatalf("Top=%d want scrolled past frame/Dump header", w.viewport.Top)
	}
}

func TestAssemblyWidgetBrowsePreservesScreenRow(t *testing.T) {
	w := NewAssemblyWidget()
	items := make([]models.AsmLine, 40)
	for i := range items {
		items[i] = models.AsmLine{Addr: fmt.Sprintf("0x%x", 0x1000+i*4), Inst: "nop"}
	}
	w.SetItems(items, items[0].Addr, items[35].Addr, "")
	w.browsePreserveRow = 3
	w.revealSel()
	if got := w.viewport.CursorLine - w.viewport.Top; got != 3 {
		t.Fatalf("screen row=%d want 3 (Top=%d Cursor=%d)", got, w.viewport.Top, w.viewport.CursorLine)
	}
	if w.browsePreserveRow != -1 {
		t.Fatalf("preserve row should clear after reveal")
	}
}

func TestAssemblyWidgetShowsEndAndRestoresHeader(t *testing.T) {
	w := NewAssemblyWidget()
	w.SetContext([]string{"#5  0x7ffff7e5791e in _IO_do_write () from /usr/lib64/libc.so.6"})
	items := make([]models.AsmLine, 30)
	for i := range items {
		inst := "nop"
		if i == len(items)-1 {
			inst = "ret"
		}
		items[i] = models.AsmLine{
			Addr:   fmt.Sprintf("0x%x", 0x7ffff7e57900+i*4),
			Inst:   inst,
			Offset: fmt.Sprintf("%d", i*4),
		}
	}
	w.SetItems(items, items[5].Addr, items[5].Addr, "_IO_do_write")
	endIdx := -1
	for i := 0; i < w.buf.NumLines(); i++ {
		if strings.Contains(w.buf.Line(i), "End of assembler dump.") {
			endIdx = i
			break
		}
	}
	if endIdx < 0 {
		t.Fatal("missing End of assembler dump.")
	}
	// Scroll to last insn — End must be on-screen under ret.
	for w.selIdx < len(items)-1 {
		w.MoveSel(1)
	}
	h := 20
	if endIdx < w.viewport.Top || endIdx >= w.viewport.Top+h {
		t.Fatalf("End line %d not visible (Top=%d h=%d)", endIdx, w.viewport.Top, h)
	}
	// Scroll back to first insn — frame + Dump header return.
	for w.selIdx > 0 {
		w.MoveSel(-1)
	}
	if w.viewport.Top != 0 {
		t.Fatalf("Top=%d want 0 so frame/Dump header is visible", w.viewport.Top)
	}
	if !strings.Contains(w.buf.Line(0), "#5  0x7ffff7e5791e in _IO_do_write") {
		t.Fatalf("frame header missing: %q", w.buf.Line(0))
	}
	if !strings.Contains(w.buf.Line(1), "Dump of assembler code for function _IO_do_write") {
		t.Fatalf("Dump header missing: %q", w.buf.Line(1))
	}
}

func TestAssemblyWidgetKeepsPreambleVisible(t *testing.T) {
	w := NewAssemblyWidget()
	w.SetContext([]string{
		"#0  a", "#1  b", "#2  write", "#3  c", "#4  d", "#5  e", "#6  f", "#7  g", "#8  main",
	})
	items := []models.AsmLine{
		{Addr: "0x1000", Inst: "nop", Offset: "0"},
		{Addr: "0x1004", Inst: "nop", Offset: "4"},
		{Addr: "0x1008", Inst: "ret", Offset: "8"},
	}
	w.SetItems(items, "0x1000", "0x1000", "")
	if w.viewport.Top != 0 {
		t.Fatalf("Top=%d want 0 so stack preamble stays visible", w.viewport.Top)
	}
	if !strings.Contains(w.buf.Line(0), "#0  a") {
		t.Fatalf("preamble missing: %q", w.buf.Line(0))
	}
	// Windowed/?? view: no Dump header from a nearby symbol name.
	for i := 0; i < w.buf.NumLines(); i++ {
		if strings.Contains(w.buf.Line(i), "Dump of assembler") {
			t.Fatalf("unexpected dump header on windowed view: %q", w.buf.Line(i))
		}
	}
}
