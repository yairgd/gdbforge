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

	height := c.H()

	for row := 0; row < height; row++ {

		line := v.Top + row
		if line >= v.Buffer.NumLines() {
			break
		}

		text := v.Buffer.Line(line)

		if v.Left < len(text) {
			text = text[v.Left:]
		} else {
			text = ""
		}
		c.Print(0, row, tcell.StyleDefault, text)
	}
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
}

func (v *Viewport) Up() {

	if v.CursorLine > 0 {
		v.CursorLine--
	}

	if v.CursorLine < v.Top {
		v.Top = v.CursorLine
	}
}

func (v *Viewport) Down() {

	if v.Buffer == nil {
		return
	}

	if v.CursorLine+1 < v.Buffer.NumLines() {
		v.CursorLine++
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

	v.CursorCol++

	if v.CursorCol >= v.Left+80 {
		v.Left++
	}
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

func (v *Viewport) EnsureVisible(width, height int) {

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
}
