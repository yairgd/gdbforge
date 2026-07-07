package termui

import (
	"github.com/gdamore/tcell/v2"
)

type NativeCursor struct{}

func (n *NativeCursor) Draw1(c Canvas, v *Viewport) {
	if v.hasSel {
		return
	}

	localX := v.CursorCol - v.Left
	localY := v.CursorLine - v.Top

	if localY < 0 || localY >= v.height || localX < 0 || localX >= v.width {
		return
	}

	ch := ' '
	if v.Buffer != nil {
		line := v.Buffer.Line(v.CursorLine)
		if v.CursorCol < len(line) {
			ch = rune(line[v.CursorCol])
		}
	}

	style := tcell.StyleDefault.
		Reverse(true).
		Bold(true)

	c.SetContent(localX, localY, ch, style)

	// מסתיר את ה-cursor של הטרמינל
	//:wc.HideNativeCursor()

}
func (n *NativeCursor) Draw(c Canvas, v *Viewport) {

	if v.hasSel {
		return
	}

	localX := v.CursorCol - v.Left
	localY := v.CursorLine - v.Top

	if localY < 0 || localY >= v.height || localX < 0 || localX >= v.width {
		return
	}

	c.ShowNativeCursor(localX, localY)
}
