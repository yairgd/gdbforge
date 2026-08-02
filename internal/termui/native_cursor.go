package termui

import (
	"github.com/gdamore/tcell/v2"
)

// NativeCursor is the default caret: tcell's real terminal cursor.
// Default style is a steady block (full-cell / inverse look).
type NativeCursor struct {
	Style tcell.CursorStyle
}

func NewNativeCursor() *NativeCursor {
	return &NativeCursor{Style: tcell.CursorStyleSteadyBlock}
}

func (n *NativeCursor) style() tcell.CursorStyle {
	if n == nil || n.Style == 0 {
		return tcell.CursorStyleSteadyBlock
	}
	return n.Style
}

// Paint implements CellCursor — places the system cursor at (x,y).
func (n *NativeCursor) Paint(c Canvas, x, y int, _ rune) {
	c.ShowNativeCursorStyle(x, y, n.style())
}

// Draw implements CursorPainter for Viewport-backed panes.
func (n *NativeCursor) Draw(c Canvas, v *Viewport) {
	if v == nil || v.hasSel || !v.cursorVisible {
		return
	}

	localX, localY, under, ok := v.cursorDrawPos()
	if !ok {
		return
	}
	n.Paint(c, localX, localY, under)
}
