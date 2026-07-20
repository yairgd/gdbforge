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

	// equalAlways, when true, rebalances split ratios after Split / DeleteFocus
	// (Vim 'equalalways'). Paint preserves current ratios; dragged sizes stick
	// until the next structural change.
	equalAlways bool

	// leafMarks are named bookmarks onto leaves (role-agnostic; callers choose names).
	leafMarks map[string]*Node

	dragSplit        *Node
	onResize         func()
	grid             *Grid
	mousePrimaryHeld bool

	geom map[*Node]layoutGeom
}

func (w *WidgetTree) SetOnResize(fn func()) {
	w.onResize = fn
}

func (w *WidgetTree) SetEqualAlways(v bool) {
	w.equalAlways = v
}

func (w *WidgetTree) EqualAlways() bool {
	return w.equalAlways
}

// Rebalance sets every split Ratio from directional leaf weights (1/N).
// Used by :set equalalways when the user wants an immediate equalize.
func (w *WidgetTree) Rebalance() {
	if w != nil && w.root != nil {
		ComputeRatios(w.root)
	}
}

// Root returns the tree root (for tests / layout inspection).
func (w *WidgetTree) Root() *Node {
	if w == nil {
		return nil
	}
	return w.root
}

func (w *WidgetTree) SetInsertActive(active bool) {
	w.insertActive = active
}

// FocusedWidget returns the leaf widget that currently has focus.
func (w *WidgetTree) FocusedWidget() Widget {
	leaf := w.FocusedLeaf()
	if leaf == nil {
		return nil
	}
	return leaf.Widget
}

// FocusedLeaf returns the focused leaf node (walks into First if focus is a split).
func (w *WidgetTree) FocusedLeaf() *Node {
	if w.focus == nil {
		return nil
	}
	n := w.focus
	for n != nil && n.Type == NodeSplit {
		n = n.First
	}
	if n == nil || n.Type != NodeLeaf {
		return nil
	}
	return n
}

// FocusWidget sets focus to the leaf that currently shows widget.
// Does not require layout geometry (safe at startup before BuildLayout).
func (w *WidgetTree) FocusWidget(widget Widget) bool {
	if w == nil || widget == nil {
		return false
	}
	for _, leaf := range CollectLeaves(w.root) {
		if leaf.Widget == widget {
			w.focus = leaf
			return true
		}
	}
	return false
}

// FocusLeaf sets focus to leaf if it belongs to this tree.
func (w *WidgetTree) FocusLeaf(leaf *Node) bool {
	if w == nil || leaf == nil || leaf.Type != NodeLeaf {
		return false
	}
	for _, n := range CollectLeaves(w.root) {
		if n == leaf {
			w.focus = leaf
			return true
		}
	}
	return false
}

// FindLeaf returns the first leaf for which match returns true.
func (w *WidgetTree) FindLeaf(match func(Widget) bool) *Node {
	if w == nil || match == nil {
		return nil
	}
	for _, leaf := range CollectLeaves(w.root) {
		if match(leaf.Widget) {
			return leaf
		}
	}
	return nil
}

// SetLeafMark bookmarks leaf under name. Passing a nil leaf clears the mark.
// No-op if leaf is not a leaf node in this tree.
func (w *WidgetTree) SetLeafMark(name string, leaf *Node) {
	if w == nil || name == "" {
		return
	}
	if leaf == nil {
		if w.leafMarks != nil {
			delete(w.leafMarks, name)
		}
		return
	}
	if leaf.Type != NodeLeaf || !w.containsLeaf(leaf) {
		return
	}
	if w.leafMarks == nil {
		w.leafMarks = make(map[string]*Node)
	}
	w.leafMarks[name] = leaf
}

// LeafMark returns the leaf bookmarked as name, or nil if missing or no longer
// in this tree (stale marks are cleared).
func (w *WidgetTree) LeafMark(name string) *Node {
	if w == nil || name == "" || w.leafMarks == nil {
		return nil
	}
	leaf := w.leafMarks[name]
	if leaf == nil || leaf.Type != NodeLeaf || !w.containsLeaf(leaf) {
		delete(w.leafMarks, name)
		return nil
	}
	return leaf
}

func (w *WidgetTree) containsLeaf(leaf *Node) bool {
	for _, n := range CollectLeaves(w.root) {
		if n == leaf {
			return true
		}
	}
	return false
}

// TopLeftLeaf returns the leaf nearest the top-left of the layout.
// Before geometry exists, returns the first DFS leaf (left/top in default trees).
func (w *WidgetTree) TopLeftLeaf() *Node {
	if w == nil {
		return nil
	}
	leaves := CollectLeaves(w.root)
	if len(leaves) == 0 {
		return nil
	}
	best := leaves[0]
	bestR := w.leafRect(best)
	if bestR.W() == 0 && bestR.H() == 0 {
		return best
	}
	for _, n := range leaves[1:] {
		r := w.leafRect(n)
		if r.Y() < bestR.Y() || (r.Y() == bestR.Y() && r.X() < bestR.X()) {
			best = n
			bestR = r
		}
	}
	return best
}

// ReplaceFocusedWidget swaps the widget on the focused leaf in O(1).
// Tree structure and geometry are unchanged. Returns false if there is no leaf
// or widget is nil.
func (w *WidgetTree) ReplaceFocusedWidget(widget Widget) bool {
	if widget == nil {
		return false
	}
	leaf := w.FocusedLeaf()
	if leaf == nil {
		return false
	}
	leaf.SetWidget(widget)
	return true
}

// ReplaceMatchingLeafWidget sets widget on the first non-focused leaf for which
// match returns true. Used to update a matching pane without stealing focus from
// the currently focused leaf. Returns false if no matching leaf is found.
func (w *WidgetTree) ReplaceMatchingLeafWidget(widget Widget, match func(Widget) bool) bool {
	if widget == nil || match == nil {
		return false
	}
	focus := w.FocusedLeaf()
	for _, leaf := range CollectLeaves(w.root) {
		if leaf == focus {
			continue
		}
		if !match(leaf.Widget) {
			continue
		}
		leaf.SetWidget(widget)
		return true
	}
	return false
}

func NewWidgetTree(newWidget Widget) *WidgetTree {
	node := &Node{Type: NodeLeaf, Widget: newWidget, Ratio: 1, parent: nil}

	return &WidgetTree{
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

	// When equalalways is on, rebalance to even sizes after the structural change.
	if w.equalAlways {
		ComputeRatios(w.root)
	}
}

func (l *WidgetTree) HandleEvent(ev tcell.Event) {
	if me, ok := ev.(*tcell.EventMouse); ok {
		if l.handleMouse(me) {
			return
		}
		// Wheel / middle-click: deliver to the leaf under the pointer
		// (not only the focused one). Middle-click paste is Linux-terminal style.
		if me.Buttons()&(tcell.WheelUp|tcell.WheelDown|tcell.ButtonMiddle) != 0 {
			mx, my := me.Position()
			for _, n := range CollectLeaves(l.root) {
				if l.leafRect(n).Contains(mx, my) {
					n.Widget.HandleEvent(ev)
					return
				}
			}
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
		if t.equalAlways {
			ComputeRatios(t.root)
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
	if t.equalAlways {
		ComputeRatios(t.root)
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
	WalkLeaves(l.root, func(n *Node) {
		if f, ok := n.Widget.(Focusable); ok {
			f.SetFocused(n == l.focus)
		}
		n.Widget.Draw(l.leafCanvas(n))
	})
	WalkLeaves(l.root, func(n *Node) {
		ClearStatusLine(l.leafCanvas(n))
	})
	l.redrawGrid(l.root, c)
	c.DrawHorizontalLocal(c.H(), 0, c.W(), false)
	c.DrawVerticalLocal(c.W()-1, 0, c.H(), false)

	WalkLeaves(l.root, func(n *Node) {
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
	// Ratios are applied as-is; equalalways rebalances only on Split/DeleteFocus.
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
