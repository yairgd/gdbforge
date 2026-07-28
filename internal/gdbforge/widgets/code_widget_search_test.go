package widgets

import "testing"

func TestCodeWidgetSearchJump(t *testing.T) {
	w := NewCodeWidget()
	w.rawLines = []string{
		"alpha",
		"beta foo",
		"gamma",
		"foo again",
		"omega",
	}
	// Rebuild buffer so Viewport search sees gutter+text lines.
	w.hiLines = append([]string(nil), w.rawLines...)
	w.rebuildBuffer()
	w.selLine = 1
	w.viewport.CursorLine = 0

	w.CommitSearch("foo")
	if w.SearchPattern() != "foo" {
		t.Fatalf("pattern %q", w.SearchPattern())
	}
	if w.SelLine() != 2 {
		t.Fatalf("first match want line 2, got %d", w.SelLine())
	}

	if !w.SearchNext() || w.SelLine() != 4 {
		t.Fatalf("SearchNext want 4, got %d", w.SelLine())
	}
	if !w.SearchNext() || w.SelLine() != 2 {
		t.Fatalf("SearchNext wrap want 2, got %d", w.SelLine())
	}
	if !w.SearchPrev() || w.SelLine() != 4 {
		t.Fatalf("SearchPrev want 4, got %d", w.SelLine())
	}

	w.SetSearchPattern("zzz")
	w.RevertSearch()
	if w.SearchPattern() != "foo" {
		t.Fatalf("RevertSearch want foo, got %q", w.SearchPattern())
	}
}

func TestCodeWidgetWordAtCursor(t *testing.T) {
	w := NewCodeWidget()
	w.rawLines = []string{
		"int hello_world = 1;",
		"  return hello_world;",
	}
	w.hiLines = append([]string(nil), w.rawLines...)
	w.rebuildBuffer()
	w.selLine = 1
	w.viewport.CursorLine = 0
	w.setCursorContentCol(0)
	if got := w.WordAtCursor(); got != "int" {
		t.Fatalf("WordAtCursor=%q want int", got)
	}
	// Move onto hello_world (after "int ").
	w.setCursorContentCol(4)
	if got := w.WordAtCursor(); got != "hello_world" {
		t.Fatalf("col4 WordAtCursor=%q want hello_world", got)
	}
	w.MoveCol(1)
	if got := w.WordAtCursor(); got != "hello_world" {
		t.Fatalf("after MoveCol WordAtCursor=%q", got)
	}
}
