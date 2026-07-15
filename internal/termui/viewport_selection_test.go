package termui

import (
	"fmt"
	"testing"

	"github.com/yairgd/cgdb-go/internal/platform"
)

func TestSelectionSurvivesScroll(t *testing.T) {
	buf := platform.NewBuffer()
	for i := 0; i < 40; i++ {
		buf.AppendLine(fmt.Sprintf("line %d", i))
	}
	v := NewViewport(buf)
	v.width = 80
	v.height = 10
	v.ScrollToBottom()

	v.selAnchor = bufferPos{line: 35, col: 0}
	v.selCursor = bufferPos{line: 38, col: 4}
	v.hasSel = true

	startTop := v.Top
	v.ViewScrollLineUp()
	if !v.hasSel {
		t.Fatal("selection mark should survive scroll")
	}
	if v.Top >= startTop {
		t.Fatalf("expected Top to decrease: %d -> %d", startTop, v.Top)
	}
	if v.selAnchor.line != 35 || v.selCursor.line != 38 {
		t.Fatalf("selection endpoints moved: %+v %+v", v.selAnchor, v.selCursor)
	}
}

func TestDragScrollExtendsSelection(t *testing.T) {
	buf := platform.NewBuffer()
	for i := 0; i < 40; i++ {
		buf.AppendLine(fmt.Sprintf("line %d", i))
	}
	v := NewViewport(buf)
	v.width = 80
	v.height = 10
	v.ScrollToBottom()

	v.selActive = true
	v.selAnchor = bufferPos{line: 36, col: 0}
	v.selCursor = bufferPos{line: 36, col: 0}
	v.CursorLine = 36
	v.CursorCol = 0

	v.ViewScrollLineUp()
	if !v.hasSel {
		t.Fatal("expected selection while dragging")
	}
	if v.selCursor.line != v.Top {
		t.Fatalf("drag scroll should pin selCursor to revealed top edge: got line %d top %d", v.selCursor.line, v.Top)
	}
}

func TestClipboardCopyShared(t *testing.T) {
	buf := platform.NewBuffer()
	buf.AppendLine("hello world")
	v := NewViewport(buf)
	var got string
	v.SetClipboard(ClipboardIO{
		Copy: func(s string) { got = s },
	})
	v.selAnchor = bufferPos{line: 0, col: 0}
	v.selCursor = bufferPos{line: 0, col: 5}
	v.hasSel = true
	v.CopySelection()
	if got != "hello" {
		t.Fatalf("copy: got %q want %q", got, "hello")
	}
}
