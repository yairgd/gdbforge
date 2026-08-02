package termui

// CellCursor paints a caret at local widget coordinates.
// Widgets choose an implementation (system block, inverse cell, bar, …).
type CellCursor interface {
	Paint(c Canvas, x, y int, under rune)
}

// CursorPainter paints a Viewport's logical caret (line/col → local cell).
type CursorPainter interface {
	Draw(c Canvas, vp *Viewport)
}
