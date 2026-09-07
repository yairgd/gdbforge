package termui

import (
	"testing"
	"unicode/utf8"

	"github.com/yairgd/gdbforge/internal/platform"
)

func TestViewportSearchSubstring(t *testing.T) {
	buf := platform.NewBuffer()
	buf.AppendLine("alpha")
	buf.AppendLine("beta foo")
	buf.AppendLine("gamma")
	buf.AppendLine("foo again")
	v := NewScrollDocument(buf)
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
	v := NewScrollDocument(buf)
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
	v := NewScrollDocument(buf)
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

func TestViewportCursorInSearchMatchKeepsSubstring(t *testing.T) {
	buf := platform.NewBuffer()
	buf.AppendLine("hello, gdbforge 1052945")
	buf.AppendLine("hello, gdbforge 1052946")
	buf.AppendLine("hello, gdbforge 1052947")
	v := NewScrollDocument(buf)
	v.SetSearchContentOffset(0)
	v.CursorLine = 0
	v.CommitSearch("46")
	if v.CursorLine != 1 {
		t.Fatalf("CommitSearch line=%d want 1", v.CursorLine)
	}
	if !v.CursorInSearchMatch() {
		t.Fatal("caret should sit on /46 highlight inside 1052946")
	}
	// Word under caret is the full number — */# must not expand while on match.
	if got := v.WordAtCursor(); got != "1052946" {
		t.Fatalf("WordAtCursor=%q want 1052946", got)
	}
	if !v.runeInSearchMatch(1, utf8.RuneCountInString("hello, gdbforge 10529")) {
		t.Fatal("expected '4' of 46 to be in match")
	}
	if v.runeInSearchMatch(1, 0) {
		t.Fatal("leading 'h' must not be highlighted for /46")
	}
}

func TestViewportSearchLeavesFollowTail(t *testing.T) {
	buf := platform.NewBuffer()
	for i := 0; i < 30; i++ {
		buf.AppendLine("line")
	}
	buf.AppendLine("needle here")
	buf.AppendLine("tail")
	v := NewScrollDocument(buf)
	v.SetFollowTail(true)
	v.width = 40
	v.height = 10
	v.ScrollToBottom()
	if !v.FollowTail() {
		t.Fatal("expected follow-tail before search")
	}
	v.CommitSearch("needle")
	if v.FollowTail() {
		t.Fatal("search jump must leave follow-tail")
	}
	if v.CursorLine != 30 {
		t.Fatalf("CursorLine=%d want 30", v.CursorLine)
	}
	// Draw must not yank back to the tail while follow is off.
	g := NewGrid(40, 10)
	c := NewCanvas(g).WithRect(NewRect(0, 0, 40, 10))
	v.Draw(c)
	if v.CursorLine != 30 || v.FollowTail() {
		t.Fatalf("after Draw CursorLine=%d follow=%v", v.CursorLine, v.FollowTail())
	}
}

func TestScrollDocumentFollowTailAndScrollRespectsLeave(t *testing.T) {
	buf := platform.NewBuffer()
	doc := NewScrollDocument(buf)
	doc.SetFollowTail(true)
	for i := 0; i < 20; i++ {
		buf.AppendLine("line")
	}
	doc.ScrollToBottom()
	doc.CommitSearch("line")
	if doc.FollowTail() {
		t.Fatal("search should leave follow-tail")
	}
	buf.AppendLine("new")
	if doc.FollowTail() {
		t.Fatal("append must not re-arm follow-tail after search")
	}
}
