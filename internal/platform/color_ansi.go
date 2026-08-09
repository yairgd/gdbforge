package platform

import (
	tcell "github.com/gdamore/tcell/v2"
)

// ContrastColor returns black or white for text on background c.
func ContrastColor(c tcell.Color) tcell.Color {
	r, g, b := c.RGB()
	if (int(r)+int(g)+int(b))/3 > 140 {
		return tcell.ColorBlack
	}
	return tcell.ColorWhite
}
