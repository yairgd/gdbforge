package termui

import "github.com/gdamore/tcell/v2"

// StyledCell is one terminal cell: rune + style (foreground and background).
type StyledCell struct {
	Rune  rune
	Style tcell.Style
}

// CellBuffer is a pane-sized off-screen grid blitted into a Canvas.
type CellBuffer struct {
	W, H  int
	cells []StyledCell
}

func NewCellBuffer(w, h int) *CellBuffer {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	b := &CellBuffer{W: w, H: h}
	b.cells = make([]StyledCell, w*h)
	return b
}

func (b *CellBuffer) EnsureSize(w, h int) {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	if b.W == w && b.H == h && b.cells != nil {
		return
	}
	b.W = w
	b.H = h
	b.cells = make([]StyledCell, w*h)
}

func (b *CellBuffer) index(x, y int) int {
	return y*b.W + x
}

func (b *CellBuffer) inBounds(x, y int) bool {
	return x >= 0 && x < b.W && y >= 0 && y < b.H
}

func (b *CellBuffer) Set(x, y int, ch rune, style tcell.Style) {
	if !b.inBounds(x, y) {
		return
	}
	i := b.index(x, y)
	b.cells[i].Rune = ch
	b.cells[i].Style = style
}

func (b *CellBuffer) Get(x, y int) (StyledCell, bool) {
	if !b.inBounds(x, y) {
		return StyledCell{}, false
	}
	return b.cells[b.index(x, y)], true
}

func (b *CellBuffer) Clear(style tcell.Style) {
	for i := range b.cells {
		b.cells[i].Rune = ' '
		b.cells[i].Style = style
	}
}

// BlitTo copies the buffer into c, clipped to the canvas bounds.
func (b *CellBuffer) BlitTo(c Canvas, dstX, dstY int) {
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			cell := b.cells[b.index(x, y)]
			ch := cell.Rune
			if ch == 0 {
				ch = ' '
			}
			c.SetContent(dstX+x, dstY+y, ch, cell.Style)
		}
	}
}
