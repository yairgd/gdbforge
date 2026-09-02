package termui

type rectPos struct {
	x, y int
}

type RectViewPort struct {
	rectPos rectPos
	rect    Rect
}

func NewRectViewPort() *RectViewPort {
	return &RectViewPort{
		rectPos: rectPos{0, 0},
	}
}
