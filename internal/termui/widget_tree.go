package termui

import (
	"math"

	"github.com/gdamore/tcell/v2"
)

const DirectionWeight = 1000

const minPaneCells = 3

const sepHitPad = 1

type WidgetTree struct {
	root  *Node
	focus *Node

	dragSplit *Node
	onResize  func()
}

func (w *WidgetTree) SetOnResize(fn func()) {
	w.onResize = fn
}

func NewWidgetTree(newWidget Widget) WidgetTree {
	node := &Node{Type: NodeLeaf, Widget: newWidget, Ratio: 1, parent: nil}

	return WidgetTree{
		root:  node,
		focus: node,
	}
}

func (w *WidgetTree) Split(dir SplitDir, newWidget Widget) {

	node := w.focus

	node.Type = NodeSplit
	node.Dir = dir
	node.Ratio = 0.5

	node.First = &Node{Type: NodeLeaf, Widget: node.Widget, Ratio: 1, parent: node}
	w.focus = node.First

	node.Second = &Node{Type: NodeLeaf, Widget: newWidget, Ratio: 1, parent: node}

	node.Widget = nil
}

func (l *WidgetTree) HandleEvent(ev tcell.Event) {
	if me, ok := ev.(*tcell.EventMouse); ok {
		if l.handleMouse(me) {
			return
		}
	}

	if l.focus != nil && l.focus.Type == NodeLeaf {
		l.focus.Widget.HandleEvent(ev)
	}
}

func (l *WidgetTree) handleMouse(me *tcell.EventMouse) bool {
	mx, my := me.Position()

	if l.dragSplit != nil {
		if me.Buttons()&tcell.ButtonPrimary != 0 {
			l.updateSplitDrag(mx, my)
			return true
		}
		l.dragSplit = nil
		return true
	}

	if me.Buttons()&tcell.ButtonPrimary != 0 {
		if split := l.findSeparator(mx, my); split != nil {
			l.dragSplit = split
			l.updateSplitDrag(mx, my)
			return true
		}
		l.FocusAt(mx, my)
		return false
	}

	return false
}

func (l *WidgetTree) findSeparator(x, y int) *Node {
	var found *Node
	l.walkSplits(l.root, func(n *Node) {
		if found != nil {
			return
		}
		if n.sepRect.W() > 0 && n.sepRect.H() > 0 &&
			n.sepRect.ContainsPadded(x, y, sepHitPad) {
			found = n
		}
	})
	return found
}

func (l *WidgetTree) walkSplits(n *Node, fn func(*Node)) {
	if n == nil {
		return
	}
	if n.Type == NodeSplit {
		fn(n)
		l.walkSplits(n.First, fn)
		l.walkSplits(n.Second, fn)
	}
}

func (l *WidgetTree) updateSplitDrag(mx, my int) {
	n := l.dragSplit
	if n == nil || n.Type != NodeSplit {
		return
	}

	r := n.layoutRect
	switch n.Dir {
	case Vertical:
		avail := r.W() - 1
		if avail < minPaneCells*2 {
			return
		}
		localX := mx - r.X()
		if localX < minPaneCells {
			localX = minPaneCells
		}
		if localX > avail-minPaneCells {
			localX = avail - minPaneCells
		}
		n.Ratio = float64(localX) / float64(avail)

	case Horizontal:
		avail := r.H() - 1
		if avail < minPaneCells*2 {
			return
		}
		localY := my - r.Y()
		if localY < minPaneCells {
			localY = minPaneCells
		}
		if localY > avail-minPaneCells {
			localY = avail - minPaneCells
		}
		n.Ratio = float64(localY) / float64(avail)
	}

	if l.onResize != nil {
		l.onResize()
	}
}

func (l *WidgetTree) IsDraggingSeparator() bool {
	return l.dragSplit != nil
}

func (t *WidgetTree) FocusAt(x, y int) bool {
	var leaves []*Node
	t.root.TotLeaves(&leaves)

	for _, n := range leaves {
		if n.canvas.Rect().Contains(x, y) {
			t.focus = n
			return true
		}
	}
	return false
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (t *WidgetTree) FocusRight() {
	var leaves []*Node
	t.root.TotLeaves(&leaves)

	cur := t.focus

	var best *Node
	bestScore := math.MaxInt

	for _, n := range leaves {

		if n == cur {
			continue
		}

		// Must be to the right.
		//	if n.canvas.rect.x < cur.canvas.rect.Right() {
		//		continue
		//	}

		dx := n.canvas.rect.x - cur.canvas.rect.Right()
		if dx < 0 {
			continue
		}

		dy := abs(n.canvas.rect.CenterY() - cur.canvas.rect.CenterY())

		score := dx*DirectionWeight + dy

		if score < bestScore {
			bestScore = score
			best = n
		}
	}

	if best != nil {
		t.focus = best
	}
}

func (t *WidgetTree) FocusLeft() {
	var leaves []*Node
	t.root.TotLeaves(&leaves)

	cur := t.focus

	var best *Node
	bestScore := math.MaxInt

	for _, n := range leaves {

		if n == cur {
			continue
		}

		// Must be to the left.
		if n.canvas.rect.Right() > cur.canvas.rect.x {
			continue
		}

		dx := cur.canvas.rect.x - n.canvas.rect.Right()
		dy := abs(n.canvas.rect.CenterY() - cur.canvas.rect.CenterY())

		score := dx*DirectionWeight + dy

		if score < bestScore {
			bestScore = score
			best = n
		}
	}

	if best != nil {
		t.focus = best
	}
}

func (t *WidgetTree) FocusDown() {
	var leaves []*Node
	t.root.TotLeaves(&leaves)

	cur := t.focus

	var best *Node
	bestScore := math.MaxInt

	for _, n := range leaves {

		if n == cur {
			continue
		}

		// Must be below.
		if n.canvas.rect.y < cur.canvas.rect.Bottom() {
			continue
		}

		dy := n.canvas.rect.y - cur.canvas.rect.Bottom()
		dx := abs(n.canvas.rect.CenterX() - cur.canvas.rect.CenterX())

		score := dy*DirectionWeight + dx

		if score < bestScore {
			bestScore = score
			best = n
		}
	}

	if best != nil {
		t.focus = best
	}
}

func (t *WidgetTree) FocusUp() {
	var leaves []*Node
	t.root.TotLeaves(&leaves)

	cur := t.focus

	var best *Node
	bestScore := math.MaxInt

	for _, n := range leaves {

		if n == cur {
			continue
		}

		// Must be above.
		if n.canvas.rect.Bottom() > cur.canvas.rect.y {
			continue
		}

		dy := cur.canvas.rect.y - n.canvas.rect.Bottom()
		dx := abs(n.canvas.rect.CenterX() - cur.canvas.rect.CenterX())

		score := dy*DirectionWeight + dx

		if score < bestScore {
			bestScore = score
			best = n
		}
	}

	if best != nil {
		t.focus = best
	}
}

func (t *WidgetTree) DeleteFocus() bool {
	if t.focus == nil {
		return false
	}

	var leaves []*Node
	t.root.TotLeaves(&leaves)
	if len(leaves) <= 1 {
		return true
	}

	if t.focus.parent == nil {
		return true
	}

	parent := t.focus.parent

	var sibling *Node
	if parent.First == t.focus {
		sibling = parent.Second
	} else {
		sibling = parent.First
	}

	grand := parent.parent

	// Parent is the root.
	if grand == nil {
		t.root = sibling
		sibling.parent = nil
		t.focus = sibling
		for t.focus.Type == NodeSplit {
			t.focus = t.focus.First
		}
		return false
	}

	if grand.First == parent {
		grand.First = sibling
	} else {
		grand.Second = sibling
	}

	sibling.parent = grand

	t.focus = sibling
	for t.focus.Type == NodeSplit {
		t.focus = t.focus.First
	}
	return false
}

func VerticalOverlap(a, b Rect) bool {
	return a.Y() < b.Bottom() &&
		b.Y() < a.Bottom()
}

func HorizontalOverlap(a, b Rect) bool {
	return a.X() < b.Right() &&
		b.X() < a.Right()
}

func (l *WidgetTree) Draw(c Canvas) {
	l.draw(c, l.root)
}

func (l *WidgetTree) draw(c Canvas, node *Node) {
	if node.Type == NodeLeaf {
		node.Widget.Draw(node.canvas)
		if node == l.focus {
			c.Printf(0, 0, tcell.StyleDefault, "active")
		} else {
			c.Printf(0, 0, tcell.StyleDefault, "      ")
		}
		return
	}
	l.draw(node.First.canvas, node.First)
	l.draw(node.Second.canvas, node.Second)

}

func (l *WidgetTree) BuildLayout(c Canvas) {
	l.buildLayout(l.root, c)

}

func (l *WidgetTree) buildLayout(
	node *Node,
	c Canvas,
) {

	if node == nil {
		return
	}

	if node.Type == NodeLeaf {
		node.canvas = c
		return
	}

	switch node.Dir {

	case Vertical:
		avail := c.W() - 1
		if avail < 1 {
			return
		}

		leftW := int(float64(avail) * node.Ratio)
		if leftW < minPaneCells {
			leftW = minPaneCells
		}
		if leftW > avail-minPaneCells {
			leftW = avail - minPaneCells
		}
		node.Ratio = float64(leftW) / float64(avail)
		rightW := c.W() - leftW - 1

		c.DrawVerticalLocal(leftW, 0, c.H(), false)

		cr := c.Rect()
		node.layoutRect = cr
		node.sepRect = NewRect(c.ScreenX(leftW), c.ScreenY(0), 1, c.H())

		r1 := c.ChildRect(0, 0, leftW, c.H())
		l.buildLayout(node.First, c.WithRect(r1))

		r2 := c.ChildRect(leftW+1, 0, rightW, c.H())
		l.buildLayout(node.Second, c.WithRect(r2))

	case Horizontal:
		avail := c.H() - 1
		if avail < 1 {
			return
		}

		topH := int(float64(avail) * node.Ratio)
		if topH < minPaneCells {
			topH = minPaneCells
		}
		if topH > avail-minPaneCells {
			topH = avail - minPaneCells
		}
		node.Ratio = float64(topH) / float64(avail)
		bottomH := c.H() - topH - 1

		c.DrawHorizontalLocal(topH, 0, c.W(), false)

		cr := c.Rect()
		node.layoutRect = cr
		node.sepRect = NewRect(c.ScreenX(0), c.ScreenY(topH), c.W(), 1)

		r1 := c.ChildRect(0, 0, c.W(), topH)
		l.buildLayout(node.First, c.WithRect(r1))

		r2 := c.ChildRect(0, topH+1, c.W(), bottomH)
		l.buildLayout(node.Second, c.WithRect(r2))

	}
}
