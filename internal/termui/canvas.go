package termui

import (
	"github.com/gdamore/tcell/v2"
)

type Canvas struct {
	screen tcell.Screen
	rect   Rect
	grid   *Grid
}

func (c *Canvas) Screen() tcell.Screen {
	return c.screen
}

func NewCanvas(s tcell.Screen, g *Grid) *Canvas {
	return &Canvas{
		grid:   g,
		screen: s,
	}
}

func (c Canvas) Rect() Rect { return c.rect }
func (c Canvas) W() int     { return c.rect.w }
func (c Canvas) H() int     { return c.rect.h }

func (c Canvas) ScreenX(localX int) int { return c.rect.x + localX }
func (c Canvas) ScreenY(localY int) int { return c.rect.y + localY }

func (c Canvas) ChildRect(localX, localY, w, h int) Rect {
	return NewRect(c.rect.x+localX, c.rect.y+localY, w, h)
}

func (c Canvas) SetContent(localX, localY int, ch rune, style tcell.Style) {
	c.screen.SetContent(c.ScreenX(localX), c.ScreenY(localY), ch, nil, style)
}

func (c Canvas) ShowCursor(localX, localY int) {
	c.screen.ShowCursor(c.ScreenX(localX), c.ScreenY(localY))
}

func (c Canvas) HideCursor() {
	c.screen.HideCursor()
}
func (c Canvas) Fill(ch rune, style tcell.Style) {
	for row := 0; row < c.H(); row++ {
		for col := 0; col < c.W(); col++ {
			c.SetContent(col, row, ch, style)
		}
	}
}

func (c Canvas) DrawVerticalLocal(localX, localY1, localY2 int, bold bool) {
	c.DrawVertical(c.ScreenX(localX), c.ScreenY(localY1), c.ScreenY(localY2), bold)
}

func (c Canvas) DrawHorizontalLocal(localY, localX1, localX2 int, bold bool) {
	c.DrawHorizontal(c.ScreenY(localY), c.ScreenX(localX1), c.ScreenX(localX2), bold)
}

func (c Canvas) DrawVertical(x, y1, y2 int, bold bool) {
	if c.grid == nil {
		return
	}

	for y := y1; y < y2; y++ {
		if x >= 0 && x < c.grid.W && y >= 0 && y < c.grid.H {
			cell := &c.grid.Cells[x][y]
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

func (c Canvas) DrawHorizontal(y, x1, x2 int, bold bool) {
	if c.grid == nil {
		return
	}

	for x := x1; x < x2; x++ {
		if x >= 0 && x < c.grid.W && y >= 0 && y < c.grid.H {
			cell := &c.grid.Cells[x][y]
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
