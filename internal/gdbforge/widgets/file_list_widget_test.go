package widgets

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

func TestFileListWidgetSetItems(t *testing.T) {
	w := NewFileListWidget()
	if got := w.LinesForTest(); len(got) != 1 || got[0] != "no files" {
		t.Fatalf("empty=%v", got)
	}
	w.SetItems([]string{"/tmp/a.c", "/proj/b.c"})
	lines := w.LinesForTest()
	if len(lines) != 2 || lines[0] != "/tmp/a.c" || lines[1] != "/proj/b.c" {
		t.Fatalf("lines=%v", lines)
	}
}

func TestFileListWidgetOpenEnter(t *testing.T) {
	w := NewFileListWidget()
	w.SetFocused(true)
	w.SetItems([]string{"/tmp/a.c", "/tmp/b.c"})
	w.move(1)
	var opened string
	w.OnOpen = func(path string) { opened = path }
	w.openSelected()
	if opened != "/tmp/b.c" {
		t.Fatalf("opened=%q", opened)
	}
}

func TestFileListWidgetMouseSelectThenOpen(t *testing.T) {
	w := NewFileListWidget()
	w.SetFocused(true)
	w.SetItems([]string{"/tmp/a.c", "/tmp/b.c"})
	var opened string
	w.OnOpen = func(path string) { opened = path }

	// First click on row 1: select only.
	w.viewport.CursorLine = 1
	prev := w.selected
	line := w.clampLine(w.viewport.CursorLine)
	w.selected = line
	if line == prev {
		w.openSelected()
	}
	if w.Selected() != 1 {
		t.Fatalf("selected=%d", w.Selected())
	}
	if opened != "" {
		t.Fatalf("first click must not open, got %q", opened)
	}

	// Second click on same row: open.
	prev = w.selected
	line = w.clampLine(w.viewport.CursorLine)
	w.selected = line
	if line == prev {
		w.openSelected()
	}
	if opened != "/tmp/b.c" {
		t.Fatalf("opened=%q", opened)
	}
}

func TestFileListWidgetMarkColorFromState(t *testing.T) {
	st := platform.NewAppState()
	st.SetMarkColor(tcell.ColorNavy)
	w := NewFileListWidget()
	w.SetAppState(st)
	if w.markColor() != tcell.ColorNavy {
		t.Fatalf("mark=%v", w.markColor())
	}
}
