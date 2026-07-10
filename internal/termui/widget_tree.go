package termui

import (
	"math"

	"github.com/gdamore/tcell/v2"
)

const DirectionWeight = 1000

const minPaneCells = 3

type WidgetTree struct {
	root  *Node
	focus *Node

	dragSplit        *Node
	onResize         func()
	grid             *Grid
	mousePrimaryHeld bool
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

	node.First = &Node{Type: NodeLeaf, Widget: node.Widget, Ratio: 1, parent: node}
	w.focus = node.First

	node.Second = &Node{Type: NodeLeaf, Widget: newWidget, Ratio: 1, parent: node}

	node.Widget = nil

	// Recompute every split's ratio from directional leaf counts so panes are
	// evenly sized (1/N per direction), matching the pre-drag layout. buildLayout
	// still honors node.Ratio, so separator drags remain effective after a split.
	w.root.ComputeRatios()
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
	l.walkSplits(l.root, func(n *Node) {
		if found != nil {
			return
		}
		if !n.sepRect.Contains(x, y) {
			return
		}
		if l.grid != nil && !l.grid.isSplitSeparatorCell(x, y, n.Dir) {
			return
		}
		found = n
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

	// Only the two panes adjacent to this separator should change size. Keep
	// every other pane at its current absolute size by adjusting the ratios
	// along the two edges that meet the separator.
	pinFirstEdge(n.First, n.Dir, local)
	pinSecondEdge(n.Second, n.Dir, total-local-1)

	if l.onResize != nil {
		l.onResize()
	}
}

// extent returns the node's current size (in cells) along dir, as of the last
// BuildLayout. Leaves store their region in canvas; splits store it in layoutRect.
func (n *Node) extent(dir SplitDir) int {
	var r Rect
	if n.Type == NodeLeaf {
		r = n.canvas.Rect()
	} else {
		r = n.layoutRect
	}
	if dir == Vertical {
		return r.W()
	}
	return r.H()
}

// pinFirstEdge walks the far (right/bottom) edge of a subtree that will occupy
// `region` cells along dir, keeping every First child at its current absolute
// size so only the edge-most leaf absorbs the change.
func pinFirstEdge(node *Node, dir SplitDir, region int) {
	for node != nil && node.Type == NodeSplit && node.Dir == dir {
		avail := region - 1
		if avail < minPaneCells*2 {
			return
		}
		firstAbs := node.First.extent(dir)
		if firstAbs < minPaneCells {
			firstAbs = minPaneCells
		}
		if firstAbs > avail-minPaneCells {
			firstAbs = avail - minPaneCells
		}
		node.Ratio = float64(firstAbs) / float64(avail)
		region = region - firstAbs - 1
		node = node.Second
	}
}

// pinSecondEdge is the mirror of pinFirstEdge for the near (left/top) edge:
// it keeps every Second child at its current absolute size.
func pinSecondEdge(node *Node, dir SplitDir, region int) {
	for node != nil && node.Type == NodeSplit && node.Dir == dir {
		avail := region - 1
		if avail < minPaneCells*2 {
			return
		}
		secondAbs := node.Second.extent(dir)
		if secondAbs < minPaneCells {
			secondAbs = minPaneCells
		}
		if secondAbs > avail-minPaneCells {
			secondAbs = avail - minPaneCells
		}
		firstW := avail - secondAbs
		node.Ratio = float64(firstW) / float64(avail)
		region = firstW
		node = node.First
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
		// Rebalance remaining panes to even sizes.
		t.root.ComputeRatios()
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
	t.root.ComputeRatios()
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
	l.drawWidgets(l.root)
	l.clearStatusRows(l.root)
	l.redrawGrid(l.root, c)
	c.DrawHorizontalLocal(c.H(), 0, c.W(), false)
	c.DrawVerticalLocal(c.W()-1, 0, c.H(), false)

	l.drawStatusLines(l.root)
}

func (l *WidgetTree) clearStatusRows(node *Node) {
	if node == nil {
		return
	}
	if node.Type == NodeLeaf {
		ClearStatusLine(node.canvas)
		return
	}
	l.clearStatusRows(node.First)
	l.clearStatusRows(node.Second)
}

func (l *WidgetTree) drawWidgets(node *Node) {
	if node == nil {
		return
	}
	if node.Type == NodeLeaf {
		node.Widget.Draw(node.canvas)
		return
	}
	l.drawWidgets(node.First)
	l.drawWidgets(node.Second)
}

func (l *WidgetTree) drawStatusLines(node *Node) {
	if node == nil {
		return
	}
	if node.Type == NodeLeaf {
		node.Widget.DrawStatusLine(node.canvas, node == l.focus)
		return
	}
	l.drawStatusLines(node.First)
	l.drawStatusLines(node.Second)
}

func (l *WidgetTree) redrawGrid(node *Node, c Canvas) {
	if node == nil || node.Type == NodeLeaf {
		return
	}

	switch node.Dir {
	case Vertical:
		leftW, _, r1, r2 := verticalSplitRects(node, c)
		c.DrawVerticalLocal(leftW, 0, c.H(), false)
		l.redrawGrid(node.First, c.WithRect(r1))
		l.redrawGrid(node.Second, c.WithRect(r2))

	case Horizontal:
		topH, _, r1, r2 := horizontalSplitRects(node, c)
		c.DrawHorizontalLocal(topH, 0, c.W(), false)
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
		leftW, _, r1, r2 := verticalSplitRects(node, c)
		avail := c.W() - 1
		if avail < 1 {
			return
		}
		node.Ratio = float64(leftW) / float64(avail)

		c.DrawVerticalLocal(leftW, 0, c.H(), false)

		cr := c.Rect()
		node.layoutRect = cr
		node.sepRect = NewRect(c.ScreenX(leftW), c.ScreenY(0), 1, c.H())

		l.buildLayout(node.First, c.WithRect(r1))
		l.buildLayout(node.Second, c.WithRect(r2))

	case Horizontal:
		topH, _, r1, r2 := horizontalSplitRects(node, c)
		avail := c.H() - 1
		if avail < 1 {
			return
		}
		node.Ratio = float64(topH) / float64(avail)

		c.DrawHorizontalLocal(topH, 0, c.W(), false)

		cr := c.Rect()
		node.layoutRect = cr
		node.sepRect = NewRect(c.ScreenX(0), c.ScreenY(topH), c.W(), 1)

		l.buildLayout(node.First, c.WithRect(r1))
		l.buildLayout(node.Second, c.WithRect(r2))

	}
}
