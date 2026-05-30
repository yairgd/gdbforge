package termui

import (
	tcell "github.com/gdamore/tcell/v2"
)

type Grid struct {
	W int
	H int

	Cells [][]Cell
}

func NewGrid(w, h int) *Grid {
	g := &Grid{
		W: w,
		H: h,
	}

	g.Cells = make([][]Cell, w+0)

	for x := 0; x < w+0; x++ {
		g.Cells[x] = make([]Cell, h)
	}

	return g
}

/*
Add vertical separator.
*/
func (g *Grid) DrawVertical(x, y1, y2 int, bold bool) {

	for y := y1; y < y2; y++ {
		if y >= 0 && y < len(g.Cells) {
			cell := &g.Cells[x][y]
			cell.Bold = bold
			if y == y1 {
				cell.Down = true
			} else if y == y2-1 {
				cell.Up = true
			} else {
				cell.Up = true
				cell.Down = true
			}
		}
	}
}

/*
Add horizontal separator.
*/
func (g *Grid) DrawHorizontal(
	y int,
	x1 int,
	x2 int,
	bold bool,
) {

	for x := x1; x < x2; x++ {
		if x >= 0 && x < len(g.Cells) {
			cell := &g.Cells[x][y]
			cell.Bold = bold

			if x == x1 {
				cell.Right = true
			} else if x == x2-1 {
				cell.Left = true
			} else {
				cell.Left = true
				cell.Right = true
			}
		}
	}
}

func (g *Grid) Draw(
	screen tcell.Screen,
	style tcell.Style,
) {

	for y := 0; y < g.H; y++ {

		for x := 0; x < g.W; x++ {
			g.Cells[x][y].EdgesToRune()

			r := g.Cells[x][y].Rune
			if r != ' ' {
				screen.SetContent(
					x,
					y,
					r,
					nil,
					style,
				)
			}
		}
	}

}
