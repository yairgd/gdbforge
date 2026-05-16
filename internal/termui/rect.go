package termui

type Rect struct {
	x, y, w, h int
}

func NewRect(x, y, w, h int) Rect {
	return Rect{x, y, w, h}
}
