package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestCanvasClearLineStaysInRect(t *testing.T) {
	g := NewGrid(20, 5)
	full := Canvas{rect: NewRect(0, 0, 20, 5), grid: g}
	left := full.WithRect(NewRect(0, 0, 9, 5))
	right := full.WithRect(NewRect(10, 0, 9, 5))

	st := tcell.StyleDefault
	for x := 0; x < 9; x++ {
		left.SetContent(x, 1, 'L', st)
		right.SetContent(x, 1, 'R', st)
	}

	right.ClearLine(1, st)

	for x := 0; x < 9; x++ {
		if g.Cells[x][1].Rune != 'L' {
			t.Fatalf("left col %d wiped by right ClearLine: %q", x, g.Cells[x][1].Rune)
		}
	}
	for x := 10; x < 19; x++ {
		if g.Cells[x][1].Rune != ' ' {
			t.Fatalf("right col %d not cleared: %q", x, g.Cells[x][1].Rune)
		}
	}
}
