package termui

import (
	"testing"

	"github.com/yairgd/gdbforge/internal/platform"
)

func TestViewportSearchSubstring(t *testing.T) {
	buf := platform.NewBuffer()
	buf.AppendLine("alpha")
	buf.AppendLine("beta foo")
	buf.AppendLine("gamma")
	buf.AppendLine("foo again")
	v := NewViewport(buf)
	v.SetSearchContentOffset(0)
	v.CursorLine = 0

	v.CommitSearch("foo")
	if v.SearchPattern() != "foo" || v.CursorLine != 1 {
		t.Fatalf("commit pattern=%q line=%d", v.SearchPattern(), v.CursorLine)
	}
	if !v.runeInSearchMatch(1, 5) || v.runeInSearchMatch(1, 4) {
		t.Fatal("substring match columns")
	}
	if !v.SearchNext() || v.CursorLine != 3 {
		t.Fatalf("next line=%d", v.CursorLine)
	}
	v.SetSearchPattern("zzz")
	v.RevertSearch()
	if v.SearchPattern() != "foo" {
		t.Fatal("revert")
	}
}

func TestViewportWordAtCursor(t *testing.T) {
	buf := platform.NewBuffer()
	buf.AppendLine("  hello_world  next")
	buf.AppendLine("foo bar")
	v := NewViewport(buf)
	v.SetSearchContentOffset(0)
	v.CursorLine = 0
	v.CursorCol = 4 // on 'l' of hello
	if got := v.WordAtCursor(); got != "hello_world" {
		t.Fatalf("WordAtCursor=%q", got)
	}
	v.CursorCol = 0 // leading spaces → first identifier
	if got := v.WordAtCursor(); got != "hello_world" {
		t.Fatalf("col0 WordAtCursor=%q", got)
	}
}

func TestViewportWordAtCursorSkipsPunctBanner(t *testing.T) {
	buf := platform.NewBuffer()
	buf.AppendLine("=== sdk_cpp_demo done ===")
	buf.AppendLine("call sdk_cpp_demo()")
	v := NewViewport(buf)
	v.SetSearchContentOffset(0)
	v.CursorLine = 0
	v.CursorCol = 0
	if got := v.WordAtCursor(); got != "sdk_cpp_demo" {
		t.Fatalf("banner WordAtCursor=%q want sdk_cpp_demo", got)
	}
	// On the trailing === still prefer nearest identifier.
	v.CursorCol = len("=== sdk_cpp_demo done ===") - 1
	if got := v.WordAtCursor(); got != "done" {
		t.Fatalf("trailing === WordAtCursor=%q want done", got)
	}
}
