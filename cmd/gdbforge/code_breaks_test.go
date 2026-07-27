package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
)

// Reused CodeWidgets (remotegdb / Clear / prior :e) must still get BP gutters
// from the shared model when showCodeAt runs — not only on first create.
func TestShowCodeAtReusedBufferPaintsBreaks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	if err := os.WriteFile(path, []byte("int main(void) {\n  return 0;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := &DebuggerApp{
		debug:       debugstate.New(nil),
		breakpoints: &models.BreakpointList{},
		fileBuffers: make(map[string]*widgets.CodeWidget),
	}

	w := widgets.NewCodeWidget()
	a.wireCodeWidget(w)
	if err := w.ShowLocation(path, 1); err != nil {
		t.Fatal(err)
	}
	w.Clear() // empty gutters like clearCodePane after inferior exit
	norm := normalizeCodePath(path)
	a.fileBuffers[norm] = w

	a.breakpoints.MergeFromGDB([]models.BreakInfo{
		{Number: 1, File: path, Line: 2, Enabled: true},
	})

	got := a.showCodeAt(path, 1)
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

	a := &DebuggerApp{
		debug:       debugstate.New(nil),
		breakpoints: &models.BreakpointList{},
		fileBuffers: make(map[string]*widgets.CodeWidget),
	}

	w := widgets.NewCodeWidget()
	a.wireCodeWidget(w)
	norm := normalizeCodePath(path)
	a.fileBuffers[norm] = w

	a.breakpoints.MergeFromGDB([]models.BreakInfo{
		{Number: 1, File: path, Line: 3, Enabled: true},
	})

	got := a.showCodeBrowse(path, 2)
	if got == nil || got != w {
		t.Fatal("expected reused CodeWidget")
	}
	if !got.HasEnabledBreak(3) {
		t.Fatal("showCodeBrowse must paint BP gutters on reuse")
	}
}

func TestRefreshBreakpointsAfterStopNoMCP(t *testing.T) {
	a := &DebuggerApp{
		debug:       debugstate.New(nil),
		breakpoints: &models.BreakpointList{},
		fileBuffers: make(map[string]*widgets.CodeWidget),
	}
	// No gdbMcp — must not panic; model stays untouched.
	a.breakpoints.MergeFromGDB([]models.BreakInfo{
		{Number: 1, File: "x.c", Line: 1, Enabled: true},
	})
	a.refreshBreakpointsAfterStop()
	if a.breakpoints.Len() != 1 {
		t.Fatalf("len=%d want 1 (failed fetch must keep model)", a.breakpoints.Len())
	}
}

func TestPaintCodeBreakmarksAllBuffers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &DebuggerApp{
		debug:       debugstate.New(nil),
		breakpoints: &models.BreakpointList{},
		fileBuffers: make(map[string]*widgets.CodeWidget),
	}
	w := widgets.NewCodeWidget()
	a.wireCodeWidget(w)
	if err := w.ShowLocation(path, 1); err != nil {
		t.Fatal(err)
	}
	norm := normalizeCodePath(path)
	a.fileBuffers[norm] = w
	a.breakpoints.MergeFromGDB([]models.BreakInfo{
		{Number: 1, File: path, Line: 2, Enabled: true},
		{Number: 2, File: path, Line: 3, Enabled: true},
	})
	a.paintCodeBreakmarks(a.breakpoints.Items())
	if !w.HasEnabledBreak(2) || !w.HasEnabledBreak(3) {
		t.Fatal("paintCodeBreakmarks must mark all BPs for the file")
	}
}
