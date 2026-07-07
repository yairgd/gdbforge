package termui

type CursorPainter interface {
	Draw(c Canvas, vp *Viewport)
}
