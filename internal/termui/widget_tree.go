package termui

import (
	"math"

	"github.com/gdamore/tcell/v2"
)

const DirectionWeight = 1000

const minPaneCells = 3

// layoutGeom holds per-frame paint and hit-test geometry for a node.
// Rebuilt by BuildLayout; not stored on Node.
type layoutGeom struct {
	canvas     Canvas // leaf paint region
	layoutRect Rect   // split region (screen coords)
	sepRect    Rect   // separator hit target (screen coords)
}

type WidgetTree struct {
	root  *Node
	focus *Node

	// insertActive: true → green status on focus; false → blue (normal mode).
	insertActive bool

	dragSplit        *Node
	onResize         func()
	grid             *Grid
	mousePrimaryHeld bool

	geom map[*Node]layoutGeom
}

func (w *WidgetTree) SetOnResize(fn func()) {
	w.onResize = fn
}

func (w *WidgetTree) SetInsertActive(active bool) {
	w.insertActive = active
}

func NewWidgetTree(newWidget Widget) WidgetTree {
	node := &Node{Type: NodeLeaf, Widget: newWidget, Ratio: 1, parent: nil}

	return WidgetTree{
		root:  node,
		focus: node,
		geom:  make(map[*Node]layoutGeom),
	}
}

func (w *WidgetTree) leafCanvas(n *Node) Canvas {
	return w.geom[n].canvas
}

func (w *WidgetTree) leafRect(n *Node) Rect {
	return w.geom[n].canvas.Rect()
}

func (w *WidgetTree) extentAlong(n *Node, dir SplitDir) int {
	g := w.geom[n]
	var r Rect
	if n.Type == NodeLeaf {
		r = g.canvas.Rect()
	} else {
		r = g.layoutRect
	}
	if dir == Vertical {
		return r.W()
	}
	return r.H()
}

func (w *WidgetTree) Split(dir SplitDir, newWidget Widget) {

	node := w.focus

	node.Type = NodeSplit
	node.Dir = dir

	node.First = &Node{Type: NodeLeaf, Widget: node.Widget, Ratio: 1, parent: node}
	w.focus = node.First

	node.Second = &Node{Type: NodeLeaf, Widget: newWidget, Ratio: 1, parent: node}

	node.Widget = nil

	// Recompute every split's ratio from directional leaf counts so panes are
	// evenly sized (1/N per direction), matching the pre-drag layout. buildLayout
	// still honors node.Ratio, so separator drags remain effective after a split.
	ComputeRatios(w.root)
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
	primary := me.Buttons()&tcell.ButtonPrimary != 0

	if l.dragSplit != nil {
		if primary {
			l.updateSplitDrag(mx, my)
			return true
		}
		l.dragSplit = nil
		l.mousePrimaryHeld = false
		return true
	}

	if primary {
		if !l.mousePrimaryHeld {
			l.mousePrimaryHeld = true
			if split := l.findSeparator(mx, my); split != nil {
				l.dragSplit = split
				l.updateSplitDrag(mx, my)
				return true
			}
			l.FocusAt(mx, my)
		}
		return false
	}

	l.mousePrimaryHeld = false
	return false
}

func (l *WidgetTree) findSeparator(x, y int) *Node {
	var found *Node
	WalkSplits(l.root, func(n *Node) {
		if found != nil {
			return
		}
		if !l.geom[n].sepRect.Contains(x, y) {
			return
		}
		if l.grid != nil && !l.grid.isSplitSeparatorCell(x, y, n.Dir) {
			return
		}
		found = n
	})
	return found
}

func (l *WidgetTree) updateSplitDrag(mx, my int) {
	n := l.dragSplit
	if n == nil || n.Type != NodeSplit {
		return
	}

	r := l.geom[n].layoutRect

	var total, local int
	switch n.Dir {
	case Vertical:
		total = r.W()
		local = mx - r.X()
	case Horizontal:
		total = r.H()
		local = my - r.Y()
	}

	avail := total - 1
	if avail < minPaneCells*2 {
		return
	}
	if local < minPaneCells {
		local = minPaneCells
	}
	if local > avail-minPaneCells {
		local = avail - minPaneCells
	}

	n.Ratio = float64(local) / float64(avail)

	extent := func(node *Node) int {
		return l.extentAlong(node, n.Dir)
	}

	// Only the two panes adjacent to this separator should change size. Keep
	// every other pane at its current absolute size by adjusting the ratios
	// along the two edges that meet the separator.
	pinFirstEdge(n.First, n.Dir, local, extent)
	pinSecondEdge(n.Second, n.Dir, total-local-1, extent)

	if l.onResize != nil {
		l.onResize()
	}
}

func (l *WidgetTree) IsDraggingSeparator() bool {
	return l.dragSplit != nil
}

func (t *WidgetTree) FocusAt(x, y int) bool {
	for _, n := range CollectLeaves(t.root) {
		if t.leafRect(n).Contains(x, y) {
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
	leaves := CollectLeaves(t.root)
	cur := t.focus
	curR := t.leafRect(cur)

	var best *Node
	bestScore := math.MaxInt

	for _, n := range leaves {

		if n == cur {
			continue
		}

		nr := t.leafRect(n)
		dx := nr.x - curR.Right()
		if dx < 0 {
			continue
		}

		dy := abs(nr.CenterY() - curR.CenterY())

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
	leaves := CollectLeaves(t.root)
	cur := t.focus
	curR := t.leafRect(cur)

	var best *Node
	bestScore := math.MaxInt

	for _, n := range leaves {

		if n == cur {
			continue
		}

		nr := t.leafRect(n)
		// Must be to the left.
		if nr.Right() > curR.x {
			continue
		}

		dx := curR.x - nr.Right()
		dy := abs(nr.CenterY() - curR.CenterY())

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
	leaves := CollectLeaves(t.root)
	cur := t.focus
	curR := t.leafRect(cur)

	var best *Node
	bestScore := math.MaxInt

	for _, n := range leaves {

		if n == cur {
			continue
		}

		nr := t.leafRect(n)
		// Must be below.
		if nr.y < curR.Bottom() {
			continue
		}

		dy := nr.y - curR.Bottom()
		dx := abs(nr.CenterX() - curR.CenterX())

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
	leaves := CollectLeaves(t.root)
	cur := t.focus
	curR := t.leafRect(cur)

	var best *Node
	bestScore := math.MaxInt

	for _, n := range leaves {

		if n == cur {
			continue
		}

		nr := t.leafRect(n)
		// Must be above.
		if nr.Bottom() > curR.y {
			continue
		}

		dy := curR.y - nr.Bottom()
		dx := abs(nr.CenterX() - curR.CenterX())

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

	leaves := CollectLeaves(t.root)
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
		// Rebalance remaining panes to even sizes.
		ComputeRatios(t.root)
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
	// Rebalance remaining panes to even sizes.
	ComputeRatios(t.root)
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
	WalkLeaves(l.root, func(n *Node) {
		n.Widget.Draw(l.leafCanvas(n))
	})
	WalkLeaves(l.root, func(n *Node) {
		ClearStatusLine(l.leafCanvas(n))
	})
	l.redrawGrid(l.root, c)
	c.DrawHorizontalLocal(c.H(), 0, c.W(), false)
	c.DrawVerticalLocal(c.W()-1, 0, c.H(), false)

	WalkLeaves(l.root, func(n *Node) {
		if n != l.focus {
			return
		}
		n.Widget.DrawStatusLine(l.leafCanvas(n), l.insertActive)
	})
}

func (l *WidgetTree) redrawGrid(node *Node, c Canvas) {
	if node == nil || node.Type == NodeLeaf {
		return
	}

	switch node.Dir {
	case Vertical:
		leftW, _, r1, r2 := verticalSplitRects(node, c)
		// Extend ±1 so this bar meets parent horizontal separators
		// (otherwise it stops on the pane edge and never shares a cell).
		c.DrawVerticalLocal(leftW, -1, c.H()+1, false)
		l.redrawGrid(node.First, c.WithRect(r1))
		l.redrawGrid(node.Second, c.WithRect(r2))

	case Horizontal:
		topH, _, r1, r2 := horizontalSplitRects(node, c)
		// Extend ±1 so this bar meets parent vertical separators.
		c.DrawHorizontalLocal(topH, -1, c.W()+1, false)
		l.redrawGrid(node.First, c.WithRect(r1))
		l.redrawGrid(node.Second, c.WithRect(r2))
	}
}

func verticalSplitRects(node *Node, c Canvas) (leftW, rightW int, r1, r2 Rect) {
	avail := c.W() - 1
	if avail < 1 {
		return 0, 0, c.Rect(), c.Rect()
	}

	leftW = int(float64(avail) * node.Ratio)
	if leftW < minPaneCells {
		leftW = minPaneCells
	}
	if leftW > avail-minPaneCells {
		leftW = avail - minPaneCells
	}
	rightW = c.W() - leftW - 1

	r1 = c.ChildRect(0, 0, leftW, c.H())
	r2 = c.ChildRect(leftW+1, 0, rightW, c.H())
	return leftW, rightW, r1, r2
}

func horizontalSplitRects(node *Node, c Canvas) (topH, bottomH int, r1, r2 Rect) {
	avail := c.H() - 1
	if avail < 1 {
		return 0, 0, c.Rect(), c.Rect()
	}

	topH = int(float64(avail) * node.Ratio)
	if topH < minPaneCells {
		topH = minPaneCells
	}
	if topH > avail-minPaneCells {
		topH = avail - minPaneCells
	}
	bottomH = c.H() - topH - 1

	r1 = c.ChildRect(0, 0, c.W(), topH)
	r2 = c.ChildRect(0, topH+1, c.W(), bottomH)
	return topH, bottomH, r1, r2
}

func (l *WidgetTree) BuildLayout(c Canvas) {
	l.grid = c.grid
	l.geom = make(map[*Node]layoutGeom)
	l.buildLayout(l.root, c)
}

func (l *WidgetTree) buildLayout(node *Node, c Canvas) {
	if node == nil {
		return
	}

	if node.Type == NodeLeaf {
		l.geom[node] = layoutGeom{canvas: c}
		return
	}

	switch node.Dir {
	case Vertical:
		leftW, _, r1, r2 := verticalSplitRects(node, c)
		avail := c.W() - 1
		if avail < 1 {
			return
		}
		node.Ratio = float64(leftW) / float64(avail)
		c.DrawVerticalLocal(leftW, -1, c.H()+1, false)
		l.geom[node] = layoutGeom{
			layoutRect: c.Rect(),
			sepRect:    NewRect(c.ScreenX(leftW), c.ScreenY(0), 1, c.H()),
		}
		l.buildLayout(node.First, c.WithRect(r1))
		l.buildLayout(node.Second, c.WithRect(r2))

	case Horizontal:
		topH, _, r1, r2 := horizontalSplitRects(node, c)
		avail := c.H() - 1
		if avail < 1 {
			return
		}
		node.Ratio = float64(topH) / float64(avail)
		c.DrawHorizontalLocal(topH, -1, c.W()+1, false)
		l.geom[node] = layoutGeom{
			layoutRect: c.Rect(),
			sepRect:    NewRect(c.ScreenX(0), c.ScreenY(topH), c.W(), 1),
		}
		l.buildLayout(node.First, c.WithRect(r1))
		l.buildLayout(node.Second, c.WithRect(r2))
	}
}
