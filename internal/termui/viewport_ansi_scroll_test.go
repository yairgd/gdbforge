package termui

import (
	"strings"
	"testing"

	"github.com/yairgd/gdbx/internal/platform"
)

func TestEnsureVisibleANSIDoesNotBlankFromByteCursorCol(t *testing.T) {
	buf := platform.NewBuffer()
	// Long ANSI line: many escape bytes, few visible cells.
	line := "\x1b[38;5;81m" + strings.Repeat("x", 40) + "\x1b[0m"
	buf.AppendLine(line)
	v := NewViewport(buf)
	v.ANSI = true
	v.width = 20
	v.height = 10
	// Simulate mouse/selection storing a byte offset as CursorCol.
	v.CursorCol = len(line) - 1
	v.Left = 0
	v.EnsureVisible(v.width, v.height)
	vis := VisibleANSIWidth(line)
	if v.Left >= vis {
		t.Fatalf("Left=%d scrolled past visible width %d (blank pane)", v.Left, vis)
	}
	if v.Left != 0 && vis <= v.width {
		t.Fatalf("short line should keep Left=0, got %d", v.Left)
	}
}

func TestCenterClampsTopToViewport(t *testing.T) {
	buf := platform.NewBuffer()
	for i := 0; i < 5; i++ {
		buf.AppendLine("x")
	}
	v := NewViewport(buf)
	v.Center(4, 20)
	if v.Top != 0 {
		t.Fatalf("Top=%d want 0 for short buffer", v.Top)
	}
}
