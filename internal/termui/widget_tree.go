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
	node := &Node{Type: NodeLeaf, Widget: newWidget, Ratio: 1}

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

	node.First = &Node{Type: NodeLeaf, Widget: w.focusWidget, Ratio: 0}
	w.focus = node.First

	node.Second = &Node{Type: NodeLeaf, Widget: newWidget, Ratio: 0}

	node.Widget = nil
}

func (l *WidgetTree) HandleEvent(ev tcell.Event) {
	l.focusWidget.HandleEvent(ev)
}

func (l *WidgetTree) Draw(c Canvas) {
	l.draw(c, l.root)
}

func (l *WidgetTree) draw(c Canvas, node *Node) {
	if node.Type == NodeLeaf {
		node.Widget.Draw(node.canvas)
		return
	}
	l.draw(c, node.First)
	l.draw(c, node.Second)

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

		leftW := int(float64(c.W()) * node.Ratio)
		rightW := c.W() - leftW - 1

		c.DrawVerticalLocal(leftW, 0, c.H(), false)

		r1 := c.ChildRect(0, 0, leftW, c.H())
		l.buildLayout(node.First, Canvas{c.screen, r1, c.grid})

		r2 := c.ChildRect(leftW+1, 0, rightW, c.H())
		l.buildLayout(node.Second, Canvas{c.screen, r2, c.grid})

	case Horizontal:

		topH := int(float64(c.H()) * node.Ratio)
		bottomH := c.H() - topH - 1

		c.DrawHorizontalLocal(topH, 0, c.W(), false)

		r1 := c.ChildRect(0, 0, c.W(), topH)
		l.buildLayout(node.First, Canvas{c.screen, r1, c.grid})

		r2 := c.ChildRect(0, topH+1, c.W(), bottomH)
		l.buildLayout(node.Second, Canvas{c.screen, r2, c.grid})

	}
}
