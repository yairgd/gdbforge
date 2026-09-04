package termui

// Point is a 2D integer coordinate in content space.
type Point struct {
	X, Y int
}

// RectViewport maps a fixed window onto a larger content rectangle.
// ContentW×ContentH is the full content size; Origin is the top-left content
// cell painted at window (0,0). Not the same as screen Rect (canvas placement).
type RectViewport struct {
	ContentW, ContentH int
	Origin             Point
}

func NewRectViewport() *RectViewport {
	return &RectViewport{}
}

func (rv *RectViewport) SetContentSize(w, h int) {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	rv.ContentW = w
	rv.ContentH = h
}

func (rv *RectViewport) SetOrigin(x, y int) {
	rv.Origin.X = x
	rv.Origin.Y = y
}

// Pan moves the view origin. Negative dx shows content to the left; negative dy upward.
func (rv *RectViewport) Pan(dx, dy int) {
	rv.Origin.X += dx
	rv.Origin.Y += dy
}

// Clamp keeps Origin inside valid range for the given window (widget canvas) size.
func (rv *RectViewport) Clamp(windowW, windowH int) {
	maxX := rv.ContentW - windowW
	if maxX < 0 {
		maxX = 0
	}
	maxY := rv.ContentH - windowH
	if maxY < 0 {
		maxY = 0
	}
	rv.Origin.X = clampInt(rv.Origin.X, 0, maxX)
	rv.Origin.Y = clampInt(rv.Origin.Y, 0, maxY)
}

func (rv *RectViewport) ScrollUp()            { rv.Pan(0, -1) }
func (rv *RectViewport) ScrollDown()          { rv.Pan(0, 1) }
func (rv *RectViewport) ScrollLeft()          { rv.Pan(-1, 0) }
func (rv *RectViewport) ScrollRight()         { rv.Pan(1, 0) }
func (rv *RectViewport) ScrollPageUp(n int)   { rv.Pan(0, -n) }
func (rv *RectViewport) ScrollPageDown(n int) { rv.Pan(0, n) }
func (rv *RectViewport) ScrollHome()          { rv.Origin.Y = 0 }

func (rv *RectViewport) ScrollEnd(windowH int) {
	maxY := rv.ContentH - windowH
	if maxY < 0 {
		maxY = 0
	}
	rv.Origin.Y = maxY
}

// VisibleContentRect returns the content slice currently mapped to the window.
func (rv *RectViewport) VisibleContentRect(windowW, windowH int) (x, y, w, h int) {
	x = rv.Origin.X
	y = rv.Origin.Y
	w = windowW
	if x+w > rv.ContentW {
		w = rv.ContentW - x
	}
	if w < 0 {
		w = 0
	}
	h = windowH
	if y+h > rv.ContentH {
		h = rv.ContentH - y
	}
	if h < 0 {
		h = 0
	}
	return x, y, w, h
}

// EnsureRowVisible adjusts Origin.Y so data row cy lies in the vertical window.
func (rv *RectViewport) EnsureRowVisible(cy, windowH int) {
	if cy < rv.Origin.Y {
		rv.Origin.Y = cy
	}
	if windowH > 0 && cy >= rv.Origin.Y+windowH {
		rv.Origin.Y = cy - windowH + 1
	}
	maxY := rv.ContentH - windowH
	if maxY < 0 {
		maxY = 0
	}
	rv.Origin.Y = clampInt(rv.Origin.Y, 0, maxY)
}

// EnsureContentVisible adjusts Origin so content cell (cx, cy) lies inside the window.
func (rv *RectViewport) EnsureContentVisible(cx, cy, windowW, windowH int) {
	if cx < rv.Origin.X {
		rv.Origin.X = cx
	}
	if cy < rv.Origin.Y {
		rv.Origin.Y = cy
	}
	if windowW > 0 && cx >= rv.Origin.X+windowW {
		rv.Origin.X = cx - windowW + 1
	}
	if windowH > 0 && cy >= rv.Origin.Y+windowH {
		rv.Origin.Y = cy - windowH + 1
	}
	rv.Clamp(windowW, windowH)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
