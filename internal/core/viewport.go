package core

type Viewport struct {
	TopLine int
	Height  int
}

func (v *Viewport) VisibleLines(b *Buffer) []string {
	end := v.TopLine + v.Height

	if end > b.NumLines() {
		end = b.NumLines()
	}

	return b.GetLines(v.TopLine, end)
}

// follow like terminal
func (v *Viewport) FollowBottom(b *Buffer) {
	if b.NumLines() > v.Height {
		v.TopLine = b.NumLines() - v.Height
	} else {
		v.TopLine = 0
	}
}
