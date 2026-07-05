package termui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/platform"
)

type Viewport struct {
	Buffer *platform.Buffer

	// First visible line/column
	Top  int
	Left int

	// Logical cursor
	CursorLine int
	CursorCol  int

	// Last canvas size seen during Draw (used for scrolling).
	width  int
	height int
}

func NewViewport(buf *platform.Buffer) *Viewport {
	return &Viewport{
		Buffer: buf,
	}
}

// Draw renders the visible portion of the buffer.
func (v *Viewport) Draw(c Canvas) {
	if v.Buffer == nil {
		return
	}

	v.width = c.W()
	v.height = c.H()

	style := tcell.StyleDefault
	width := v.width
	height := v.height

	for row := 0; row < height; row++ {
		line := v.Top + row
		if line >= v.Buffer.NumLines() {
			c.ClearLine(row, style)
			continue
		}

		text := v.Buffer.Line(line)
		if v.Left < len(text) {
			text = text[v.Left:]
		} else {
			text = ""
		}

		if len(text) > width {
			text = text[:width]
		}

		visibleLen := len(text)
		if visibleLen < width {
			c.ClearLineRange(row, visibleLen, width, style)
		}
		c.Print(0, row, style, text)
	}

	v.drawCursor(c)
}

func (v *Viewport) drawCursor(c Canvas) {
	localX := v.CursorCol - v.Left
	localY := v.CursorLine - v.Top

	if localY < 0 || localY >= v.height || localX < 0 || localX >= v.width {
		return
	}

	c.ShowCursor(localX, localY)
}

func (v *Viewport) HandleEvent(ev tcell.Event) {

	key, ok := ev.(*tcell.EventKey)
	if !ok {
		return
	}

	switch key.Key() {

	case tcell.KeyUp:
		v.Up()

	case tcell.KeyDown:
		v.Down()

	case tcell.KeyLeft:
		v.LeftChar()

	case tcell.KeyRight:
		v.RightChar()

	case tcell.KeyPgUp:
		v.PageUp(10)

	case tcell.KeyPgDn:
		v.PageDown(10)

	case tcell.KeyHome:
		v.Home()

	case tcell.KeyEnd:
		v.End()
	}

	v.EnsureVisible(v.width, v.height)
}

func (v *Viewport) Up() {

	if v.CursorLine > 0 {
		v.CursorLine--
		v.clampCursorCol()
	}

	if v.CursorLine < v.Top {
		v.Top = v.CursorLine
	}
}

func (v *Viewport) Down() {

	if v.Buffer == nil {
		return
	}

	last := v.Buffer.NumLines() - 1
	if v.CursorLine < last {
		v.CursorLine++
		v.clampCursorCol()
	}

	if v.height > 0 && v.CursorLine >= v.Top+v.height {
		v.Top = v.CursorLine - v.height + 1
	}
}

func (v *Viewport) LeftChar() {

	if v.CursorCol > 0 {
		v.CursorCol--
	}

	if v.CursorCol < v.Left {
		v.Left = v.CursorCol
	}
}

func (v *Viewport) RightChar() {
	if v.Buffer != nil {
		lineLen := len(v.Buffer.Line(v.CursorLine))
		if v.CursorCol < lineLen {
			v.CursorCol++
		}
		return
	}

	v.CursorCol++
}

func (v *Viewport) PageDown(pageSize int) {

	if v.Buffer == nil {
		return
	}

	v.CursorLine += pageSize

	last := v.Buffer.NumLines() - 1
	if v.CursorLine > last {
		v.CursorLine = last
	}

	v.Top += pageSize
	if v.Top > last {
		v.Top = last
	}
}

func (v *Viewport) PageUp(pageSize int) {

	v.CursorLine -= pageSize
	if v.CursorLine < 0 {
		v.CursorLine = 0
	}

	v.Top -= pageSize
	if v.Top < 0 {
		v.Top = 0
	}
}

func (v *Viewport) Home() {

	v.CursorCol = 0
	v.Left = 0
}

func (v *Viewport) End() {

	if v.Buffer == nil {
		return
	}

	last := v.Buffer.NumLines() - 1
	if last < 0 {
		last = 0
	}

	v.CursorLine = last
	v.Top = last
}

func (v *Viewport) Center(line int, pageHeight int) {

	if v.Buffer == nil {
		return
	}

	v.CursorLine = line

	v.Top = line - pageHeight/2
	if v.Top < 0 {
		v.Top = 0
	}

	last := v.Buffer.NumLines() - 1
	if v.Top > last {
		v.Top = last
	}
}

func (v *Viewport) clampCursorCol() {
	if v.Buffer == nil {
		return
	}

	lineLen := len(v.Buffer.Line(v.CursorLine))
	if v.CursorCol > lineLen {
		v.CursorCol = lineLen
	}
}

func (v *Viewport) EnsureVisible(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}

	if v.CursorLine < v.Top {
		v.Top = v.CursorLine
	}

	if v.CursorLine >= v.Top+height {
		v.Top = v.CursorLine - height + 1
	}

	if v.CursorCol < v.Left {
		v.Left = v.CursorCol
	}

	if v.CursorCol >= v.Left+width {
		v.Left = v.CursorCol - width + 1
	}

	if v.Buffer == nil {
		return
	}

	last := v.Buffer.NumLines() - 1
	if last < 0 {
		return
	}

	maxTop := last
	if last >= height-1 {
		maxTop = last - height + 1
	}
	if v.Top > maxTop {
		v.Top = maxTop
	}
	if v.Top < 0 {
		v.Top = 0
	}
}
