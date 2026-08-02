package termui

import (
	"github.com/gdamore/tcell/v2"
)

// InverseCursor paints a full inverse (reverse-video) cell instead of using
// the terminal's system cursor. Useful when a widget wants a drawn caret.
type InverseCursor struct{}

func NewInverseCursor() *InverseCursor {
	return &InverseCursor{}
}

func (n *InverseCursor) Paint(c Canvas, x, y int, under rune) {
	if under == 0 {
		under = ' '
	}
	style := tcell.StyleDefault.Reverse(true).Bold(true)
	c.SetContent(x, y, under, style)
}

func (n *InverseCursor) Draw(c Canvas, v *Viewport) {
	if v == nil || v.hasSel || !v.cursorVisible {
		return
	}

	localX, localY, under, ok := v.cursorDrawPos()
	if !ok {
		return
	}
	n.Paint(c, localX, localY, under)
}
