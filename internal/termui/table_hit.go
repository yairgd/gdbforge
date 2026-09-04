package termui

// HitCell maps screen coordinates to a data row, column, and rune offset within
// that column's visible (possibly truncated) cell width.
func (w *TableWidget) HitCell(mx, my int) (row, col, runeOff int, ok bool) {
	row, onRow := w.HitDataRow(mx, my)
	if !onRow {
		return -1, -1, 0, false
	}
	lx := mx - w.screenX
	if lx < 0 || lx >= w.lastW {
		return -1, -1, 0, false
	}
	lay := w.table.Layout()
	contentX := lx + w.rv.Origin.X
	gutter := w.table.Gutter()
	x := 0
	for c := 0; c < len(lay.colWidths); c++ {
		if c > 0 {
			x += gutter
		}
		cw := lay.colWidths[c]
		if contentX >= x && contentX < x+cw {
			return row, c, contentX - x, true
		}
		x += cw
	}
	return row, -1, 0, false
}
