package termui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

type Canvas struct {
	rect Rect
	grid *Grid
}

func NewCanvas(g *Grid) *Canvas {
	return &Canvas{
		grid: g,
	}
}

func (c Canvas) Rect() Rect { return c.rect }
func (c Canvas) W() int     { return c.rect.w }
func (c Canvas) H() int     { return c.rect.h }

func (c Canvas) ScreenX(x int) int { return c.rect.x + x }
func (c Canvas) ScreenY(y int) int { return c.rect.y + y }

func (c Canvas) ChildRect(x, y, w, h int) Rect {
	return NewRect(
		c.rect.x+x,
		c.rect.y+y,
		w,
		h,
	)
}

func (c Canvas) WithRect(r Rect) Canvas {
	return Canvas{rect: r, grid: c.grid}
}

func (c Canvas) SetContent(
	x,
	y int,
	ch rune,
	style tcell.Style,
) {
	c.grid.SetContent(
		c.ScreenX(x),
		c.ScreenY(y),
		ch,
		style,
	)
}

func (c Canvas) Fill(
	ch rune,
	style tcell.Style,
) {
	for y := 0; y < c.H(); y++ {
		for x := 0; x < c.W(); x++ {
			c.SetContent(x, y, ch, style)
		}
	}
}

func (c Canvas) Print(
	x,
	y int,
	style tcell.Style,
	args ...any,
) {
	c.grid.Print(
		c.ScreenX(x),
		c.ScreenY(y),
		style,
		fmt.Sprint(args...),
	)
}

func (c Canvas) Printf(
	x,
	y int,
	style tcell.Style,
	format string,
	args ...any,
) {
	c.grid.Print(
		c.ScreenX(x),
		c.ScreenY(y),
		style,
		fmt.Sprintf(format, args...),
	)
}

func (c Canvas) DrawVerticalLocal(
	x,
	y1,
	y2 int,
	bold bool,
) {
	c.grid.DrawVertical(
		c.ScreenX(x),
		c.ScreenY(y1),
		c.ScreenY(y2),
		bold,
	)
}

func (c Canvas) DrawHorizontalLocal(
	y,
	x1,
	x2 int,
	bold bool,
) {
	c.grid.DrawHorizontal(
		c.ScreenY(y),
		c.ScreenX(x1),
		c.ScreenX(x2),
		bold,
	)
}

func (c Canvas) ClearLine(localY int, style tcell.Style) {
	c.grid.ClearLine(c.ScreenY(localY), style)
}

func (c Canvas) ShowCursor(localX, localY int) {
	c.grid.ShowCursor(
		c.ScreenX(localX),
		c.ScreenY(localY),
	)
}

func (c Canvas) HideCursor() {
	c.grid.HideCursor()
}
