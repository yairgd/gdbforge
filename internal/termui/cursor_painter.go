package termui

// CellCursor paints a caret at local widget coordinates.
// Widgets choose an implementation (system block, inverse cell, bar, …).
type CellCursor interface {
	Paint(c Canvas, x, y int, under rune)
}

// DocCursorHost supplies caret geometry for CursorPainter implementations.
type DocCursorHost interface {
	CursorDrawPos() (localX, localY int, under rune, ok bool)
	HasSelection() bool
	CursorVisible() bool
}

// CursorPainter paints a document caret (line/col → local cell).
type CursorPainter interface {
	Draw(c Canvas, host DocCursorHost)
}
