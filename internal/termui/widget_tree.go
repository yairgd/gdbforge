package termui

import (
	"github.com/gdamore/tcell/v2"
)

type WidgetTree struct {
	root        *Node
	focus       *Node
	focusWidget Widget
}

func NewWidgetTree(newWidget Widget) WidgetTree {
	node := &Node{Type: NodeLeaf, Widget: newWidget, Ratio: 1, parent: nil}

	return WidgetTree{
		root:        node,
		focus:       node,
		focusWidget: newWidget,
	}
}

func (w *WidgetTree) Split(dir SplitDir, newWidget Widget) {

	node := w.focus

	node.Type = NodeSplit
	node.Dir = dir
	node.Ratio = 0.5

	node.First = &Node{Type: NodeLeaf, Widget: w.focusWidget, Ratio: 1, parent: node}
	w.focus = node.First

	node.Second = &Node{Type: NodeLeaf, Widget: newWidget, Ratio: 1, parent: node}

	node.Widget = nil
}

func (l *WidgetTree) HandleEvent(ev tcell.Event) {
	l.focusWidget.HandleEvent(ev)
}

func (l *WidgetTree) FocusRight() {
	if l.focus != nil {
		l.focusRight(l.focus)
	}
}

func (l *WidgetTree) focusRight(node *Node) {
	if node == nil {
		return
	}

	p := node.parent
	if p == nil {
		return
	}

	if p.Dir == Vertical && p.First == node {
		n := p.Second
		for n.Type != NodeLeaf {
			n = n.First
		}
		l.focus = n
		return
	}

	l.focusRight(p)
}

func (l *WidgetTree) FocusLeft() {
	if l.focus != nil {
		l.focusLeft(l.focus)
	}
}

func (l *WidgetTree) focusLeft(node *Node) {
	if node == nil {
		return
	}

	p := node.parent
	if p == nil {
		return
	}

	if p.Dir == Vertical && p.Second == node {
		n := p.First
		for n.Type != NodeLeaf {
			n = n.Second
		}
		l.focus = n
		return
	}

	l.focusLeft(p)
}

func (l *WidgetTree) FocusUp() {
	if l.focus != nil {
		l.focusUp(l.focus)
	}
}

func (l *WidgetTree) focusUp(node *Node) {
	if node == nil {
		return
	}

	p := node.parent
	if p == nil {
		return
	}

	if p.Dir == Horizontal && p.Second == node {
		n := p.First
		for n.Type != NodeLeaf {
			n = n.Second
		}
		l.focus = n
		return
	}

	l.focusUp(p)
}

func bottomMostLeaf(n *Node) *Node {
	for n.Type != NodeLeaf {
		n = n.Second
	}
	return n
}

func (l *WidgetTree) FocusDown() {
	if l.focus != nil {
		l.focusDown(l.focus)
	}
}

func (l *WidgetTree) focusDown(node *Node) {
	if node == nil {
		return
	}

	p := node.parent
	if p == nil {
		return
	}

	if p.Dir == Horizontal && p.First == node {
		n := p.Second
		for n.Type != NodeLeaf {
			n = n.First
		}
		l.focus = n
		return
	}

	l.focusDown(p)
}
func (l *WidgetTree) Draw(c Canvas) {
	l.draw(c, l.root)
}

func (l *WidgetTree) draw(c Canvas, node *Node) {
	if node.Type == NodeLeaf {
		node.Widget.Draw(node.canvas)
		if node == l.focus {
			c.Printf(0, 0, tcell.StyleDefault, "active")
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
		total := Units(node, Vertical)

		leftW := int(
			float64(c.W()) *
				float64(Units(node.First, Vertical)) /
				float64(total),
		)

		//leftW := int(float64(c.W()) * node.Ratio)
		rightW := c.W() - leftW - 1

		c.DrawVerticalLocal(leftW, 0, c.H(), false)

		r1 := c.ChildRect(0, 0, leftW, c.H())
		l.buildLayout(node.First, Canvas{c.screen, r1, c.grid})

		r2 := c.ChildRect(leftW+1, 0, rightW, c.H())
		l.buildLayout(node.Second, Canvas{c.screen, r2, c.grid})

	case Horizontal:

		total := Units(node, Horizontal)

		topH := int(
			float64(c.H()) *
				float64(Units(node.First, Horizontal)) /
				float64(total),
		)

		//topH := int(float64(c.H()) * node.Ratio)
		bottomH := c.H() - topH - 1

		c.DrawHorizontalLocal(topH, 0, c.W(), false)

		r1 := c.ChildRect(0, 0, c.W(), topH)
		l.buildLayout(node.First, Canvas{c.screen, r1, c.grid})

		r2 := c.ChildRect(0, topH+1, c.W(), bottomH)
		l.buildLayout(node.Second, Canvas{c.screen, r2, c.grid})

	}
}
