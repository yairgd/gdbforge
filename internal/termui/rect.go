package termui

type Rect struct {
	x, y, w, h int
}

func NewRect(x, y, w, h int) Rect {
	return Rect{x, y, w, h}
}

func (r Rect) X() int { return r.x }
func (r Rect) Y() int { return r.y }
func (r Rect) W() int { return r.w }
func (r Rect) H() int { return r.h }

func (r Rect) Right() int   { return r.x + r.w }
func (r Rect) Bottom() int  { return r.y + r.h }
func (r Rect) CenterY() int { return r.y + r.h/2 }
func (r Rect) CenterX() int { return r.x + r.w/2 }
