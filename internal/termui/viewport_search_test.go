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
