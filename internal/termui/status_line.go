package termui

import (
	"strings"

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

// statusSelStyle is the white highlight used while selecting status-band text.
func statusSelStyle() tcell.Style {
	return tcell.StyleDefault.
		Foreground(tcell.ColorBlack).
		Background(tcell.ColorWhite)
}

// PaintStatusBar renders the focused pane name on the bottom grid row of c.
// active selects the insert-mode (green) vs remembered (blue) style.
func PaintStatusBar(c Canvas, name string, active bool) {
	PaintStatusBarSel(c, name, active, -1, -1)
}

// PaintStatusBarSel is PaintStatusBar with an optional [selStart,selEnd) rune
// range into name highlighted (white). Negative or empty range = no highlight.
func PaintStatusBarSel(c Canvas, name string, active bool, selStart, selEnd int) {
	if name == "" || c.W() <= 0 || c.H() <= 0 {
		return
	}

	row := StatusLineRow(c)
	style := statusBarStyle(active)
	selStyle := statusSelStyle()
	if selStart > selEnd {
		selStart, selEnd = selEnd, selStart
	}
	hasSel := selStart >= 0 && selEnd > selStart

	c.ClearLineRange(row, 0, c.W(), style)

	// Prefix "▎ "
	prefix := []rune{'▎', ' '}
	col := 0
	for _, ch := range prefix {
		if col >= c.W() {
			return
		}
		c.SetContent(col, row, ch, style)
		col++
	}
	for i, ch := range []rune(name) {
		if col >= c.W() {
			break
		}
		st := style
		if hasSel && i >= selStart && i < selEnd {
			st = selStyle
		}
		c.SetContent(col, row, ch, st)
		col++
	}
}

// inactiveNameCol is the 0-based column where an unfocused pane name starts
// (4th character on the status row).
const inactiveNameCol = 3

// PaintInactiveStatusBar writes the pane name on the status row starting at
// column 4, without clearing the row so the split grid stays visible.
func PaintInactiveStatusBar(c Canvas, name string) {
	PaintInactiveStatusBarSel(c, name, -1, -1)
}

// PaintInactiveStatusBarSel is PaintInactiveStatusBar with optional highlight.
func PaintInactiveStatusBarSel(c Canvas, name string, selStart, selEnd int) {
	if name == "" || c.W() <= 0 || c.H() <= 0 {
		return
	}
	row := StatusLineRow(c)
	style := tcell.StyleDefault.Foreground(tcell.ColorGray)
	selStyle := statusSelStyle()
	name = strings.TrimSpace(name)
	if selStart > selEnd {
		selStart, selEnd = selEnd, selStart
	}
	hasSel := selStart >= 0 && selEnd > selStart
	col := inactiveNameCol
	for i, ch := range []rune(name) {
		if col >= c.W() {
			break
		}
		st := style
		if hasSel && i >= selStart && i < selEnd {
			st = selStyle
		}
		c.SetContent(col, row, ch, st)
		col++
	}
}

// ClearStatusLine resets the bottom grid row within this widget's rect only.
func ClearStatusLine(c Canvas) {
	if c.W() <= 0 || c.H() <= 0 {
		return
	}
	c.ClearLineRange(StatusLineRow(c), 0, c.W(), tcell.StyleDefault)
}
