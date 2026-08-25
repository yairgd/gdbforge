package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
)

func testAppWithBreaks() *DebuggerApp {
	a := &DebuggerApp{DebugSession: DebugSession{debug: debugstate.New(nil)}}
	a.breaks.host = a
	a.bufs.host = a
	a.bufs.initMaps()
	a.breaks.list = &models.BreakpointList{}
	return a
}

// Reused CodeWidgets (remotegdb / Clear / prior :e) must still get BP gutters
// from the shared model when showCodeAt runs — not only on first create.
func TestShowCodeAtReusedBufferPaintsBreaks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	if err := os.WriteFile(path, []byte("int main(void) {\n  return 0;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := testAppWithBreaks()

	w := widgets.NewCodeWidget()
	a.bufs.wire(w)
	if err := w.ShowLocation(path, 1); err != nil {
		t.Fatal(err)
	}
	w.Clear() // empty gutters like clearCodePane after inferior exit
	norm := normalizeCodePath(path)
	a.bufs.Buffers()[norm] = w

	a.breaks.list.MergeFromGDB([]models.BreakInfo{
		{Number: 1, File: path, Line: 2, Enabled: true},
	})

	got := a.bufs.showCodeAt(path, 1)
	if got == nil {
		t.Fatal("showCodeAt returned nil")
	}
	if got != w {
		t.Fatal("expected reused CodeWidget, not a new buffer")
	}
	if !got.HasEnabledBreak(2) {
		t.Fatal("reused buffer must paint BP gutters from BreakpointList")
	}
}

func TestShowCodeBrowseReusedBufferPaintsBreaks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := testAppWithBreaks()

	w := widgets.NewCodeWidget()
	a.bufs.wire(w)
	norm := normalizeCodePath(path)
	a.bufs.Buffers()[norm] = w

	a.breaks.list.MergeFromGDB([]models.BreakInfo{
		{Number: 1, File: path, Line: 3, Enabled: true},
	})

	got := a.bufs.showCodeBrowse(path, 2)
	if got == nil || got != w {
		t.Fatal("expected reused CodeWidget")
	}
	if !got.HasEnabledBreak(3) {
		t.Fatal("showCodeBrowse must paint BP gutters on reuse")
	}
}

func TestRefreshBreakpointsAfterStopNoMCP(t *testing.T) {
	a := testAppWithBreaks()
	// No gdbMcp — must not panic; model stays untouched.
	a.breaks.list.MergeFromGDB([]models.BreakInfo{
		{Number: 1, File: "x.c", Line: 1, Enabled: true},
	})
	a.breaks.refreshAfterStop()
	if a.breaks.list.Len() != 1 {
		t.Fatalf("len=%d want 1 (failed fetch must keep model)", a.breaks.list.Len())
	}
}

func TestPaintCodeBreakmarksAllBuffers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := testAppWithBreaks()
	w := widgets.NewCodeWidget()
	a.bufs.wire(w)
	if err := w.ShowLocation(path, 1); err != nil {
		t.Fatal(err)
	}
	norm := normalizeCodePath(path)
	a.bufs.Buffers()[norm] = w
	a.breaks.list.MergeFromGDB([]models.BreakInfo{
		{Number: 1, File: path, Line: 2, Enabled: true},
		{Number: 2, File: path, Line: 3, Enabled: true},
	})
	a.breaks.paintCodeMarks(a.breaks.Items())
	if !w.HasEnabledBreak(2) || !w.HasEnabledBreak(3) {
		t.Fatal("paintCodeBreakmarks must mark all BPs for the file")
	}
}
