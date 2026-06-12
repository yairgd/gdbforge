package termui

import (
	"github.com/gdamore/tcell/v2"
	// "google.golang.org/genproto/googleapis/type/interval"
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
