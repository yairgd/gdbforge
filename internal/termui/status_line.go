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

func statusBarStyle(active bool) tcell.Style {
	if active {
		// Insert-mode focus: strong green bar.
		return tcell.StyleDefault.
			Foreground(tcell.ColorWhite).
			Background(tcell.ColorDarkGreen).
			Bold(true)
	}
	// Normal mode / remembered pane: muted blue bar.
	return tcell.StyleDefault.
		Foreground(tcell.ColorLightCyan).
		Background(tcell.ColorNavy).
		Bold(false)
}

// PaintStatusBar renders the pane name on the bottom grid row of c.
// active selects the insert-mode (green) vs remembered (blue) style.
func PaintStatusBar(c Canvas, name string, active bool) {
	if name == "" || c.W() <= 0 || c.H() <= 0 {
		return
	}

	row := StatusLineRow(c)
	style := statusBarStyle(active)

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
