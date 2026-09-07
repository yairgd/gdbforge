package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestCellBufferSetBlitClip(t *testing.T) {
	g := NewGrid(4, 2)
	full := NewCanvas(g).WithRect(NewRect(0, 0, 4, 2))
	sub := full.WithRect(NewRect(1, 0, 2, 2))

	buf := NewCellBuffer(2, 2)
	buf.Set(0, 0, 'A', tcell.StyleDefault)
	buf.Set(1, 0, 'B', tcell.StyleDefault)
	buf.Set(0, 1, 'C', tcell.StyleDefault)
	buf.Set(1, 1, 'D', tcell.StyleDefault)
	buf.BlitTo(sub, 0, 0)

	if g.Cells[0][0].Rune != 0 && g.Cells[0][0].Rune != ' ' {
		t.Fatalf("col 0 should be untouched, got %q", g.Cells[0][0].Rune)
	}
	if g.Cells[1][0].Rune != 'A' || g.Cells[2][0].Rune != 'B' {
		t.Fatalf("row0: %q %q want AB", g.Cells[1][0].Rune, g.Cells[2][0].Rune)
	}
	if g.Cells[1][1].Rune != 'C' || g.Cells[2][1].Rune != 'D' {
		t.Fatalf("row1: %q %q want CD", g.Cells[1][1].Rune, g.Cells[2][1].Rune)
	}
}

func TestCellBufferEnsureSize(t *testing.T) {
	buf := NewCellBuffer(2, 2)
	buf.EnsureSize(3, 1)
	if buf.W != 3 || buf.H != 1 || len(buf.cells) != 3 {
		t.Fatalf("size=%dx%d cells=%d", buf.W, buf.H, len(buf.cells))
	}
}
