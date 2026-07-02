package termui

import (
	"github.com/gdamore/tcell/v2"
)

type Grid struct {
	W int
	H int

	Cells     [][]Cell
	BackCells [][]Cell

	cursorVisible bool
	cursorX       int
	cursorY       int
}

func NewGrid(w, h int) *Grid {

	g := &Grid{
		W: w,
		H: h,
	}

	g.Cells = make([][]Cell, w)
	g.BackCells = make([][]Cell, w)

	for x := range g.Cells {
		g.Cells[x] = make([]Cell, h)
		g.BackCells[x] = make([]Cell, h)
	}

	return g
}

func (g *Grid) SetContent(
	x,
	y int,
	ch rune,
	style tcell.Style,
) {

	if x < 0 || x >= g.W ||
		y < 0 || y >= g.H {
		return
	}

	cell := &g.Cells[x][y]
	cell.Style = style
	cell.Rune = ch
}

func (g *Grid) Print(
	x,
	y int,
	style tcell.Style,
	text string,
) {

	if y < 0 || y >= g.H {
		return
	}

	col := x

	for _, r := range text {

		if col >= g.W {
			break
		}

		if col >= 0 {
			g.SetContent(
				col,
				y,
				r,
				style,
			)
		}

		col++
	}
}

func (g *Grid) Clear() {

	for y := 0; y < g.H; y++ {

		for x := 0; x < g.W; x++ {

			g.Cells[x][y] = Cell{}
		}
	}
}

func (g *Grid) DrawVertical(
	x,
	y1,
	y2 int,
	bold bool,
) {

	for y := y1; y < y2; y++ {

		if x < 0 || x >= g.W ||
			y < 0 || y >= g.H {
			continue
		}

		c := &g.Cells[x][y]

		c.Bold = bold

		if y == y1 {
			c.Down = true
		} else if y == y2-1 {
			c.Up = true
		} else {
			c.Up = true
			c.Down = true
		}
	}
}

func (g *Grid) DrawHorizontal(
	y,
	x1,
	x2 int,
	bold bool,
) {

	for x := x1; x < x2; x++ {

		if x < 0 || x >= g.W ||
			y < 0 || y >= g.H {
			continue
		}

		c := &g.Cells[x][y]

		c.Bold = bold

		if x == x1 {
			c.Right = true
		} else if x == x2-1 {
			c.Left = true
		} else {
			c.Left = true
			c.Right = true
		}
	}
}

func (g *Grid) Draw(
	screen tcell.Screen,
) {

	for y := 0; y < g.H; y++ {

		for x := 0; x < g.W; x++ {

			cell := &g.Cells[x][y]

			//
			// If no explicit rune was written,
			// generate one from border edges.
			//
			if cell.Rune == 0 {
				cell.EdgesToRune()
			}

			if g.BackCells[x][y] != *cell {

				g.BackCells[x][y] = *cell

				screen.SetContent(
					x,
					y,
					cell.Rune,
					nil,
					cell.Style,
				)
			}
		}
	}
}

func (g *Grid) ClearLine(y int, style tcell.Style) {
	if y < 0 || y >= g.H {
		return
	}

	for x := 0; x < g.W; x++ {
		g.SetContent(x, y, ' ', style)
	}
}

func (g *Grid) ShowCursor(x, y int) {
	g.cursorVisible = true
	g.cursorX = x
	g.cursorY = y
}

func (g *Grid) HideCursor() {
	g.cursorVisible = false
}
