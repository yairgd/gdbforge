package termui

import (
	"fmt"

	tcell "github.com/gdamore/tcell/v2"
)

// StatusLineRow is the row where the original widgets drew "Status Line"
// (local y = c.H(), immediately below the pane content grid).
func StatusLineRow(c Canvas) int {
	return c.H()
}

// PaintStatusBar renders the pane name on the bottom grid row of c, overwriting
// whatever was drawn on that row. Uses canvas coordinates only.
func PaintStatusBar(c Canvas, name string) {
	if name == "" || c.W() <= 0 || c.H() <= 0 {
		return
	}

	row := StatusLineRow(c)
	style := tcell.StyleDefault.
		Foreground(tcell.ColorSkyblue).
		Background(tcell.ColorDarkSlateGray).
		Bold(true)

	c.ClearLineRange(row, 0, c.W(), style)

	label := fmt.Sprintf("▎ %s", name)
	for i, ch := range label {
		if i >= c.W() {
			break
		}
		c.SetContent(i, row, ch, style)
	}
}

// ClearStatusLine resets the bottom grid row within this widget's rect only.
func ClearStatusLine(c Canvas) {
	if c.W() <= 0 || c.H() <= 0 {
		return
	}
	c.ClearLineRange(StatusLineRow(c), 0, c.W(), tcell.StyleDefault)
}
