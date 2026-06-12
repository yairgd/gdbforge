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

		leftW := int(float64(c.rect.w) * node.Ratio)

		splitX := c.rect.x + leftW

		/*
			Draw separator line.
		*/
		c.grid.DrawVertical(
			splitX,
			c.rect.y-1,
			c.rect.y+c.rect.h+1,
			false,
		)

		r1 := NewRect(c.rect.x, c.rect.y, splitX-0, c.rect.h)
		l.buildLayout(node.First, Canvas{c.screen, r1, c.grid})

		r2 := NewRect(splitX+1, c.rect.y, c.rect.w-splitX-1, c.rect.h)
		l.buildLayout(node.Second, Canvas{c.screen, r2, c.grid})

	case Horizontal:

		topH := int(float64(c.rect.h) * node.Ratio)

		splitY := c.rect.y + topH

		c.grid.DrawHorizontal(
			splitY,
			c.rect.x-1,
			c.rect.x+c.rect.w+1,
			false,
		)

		r1 := NewRect(c.rect.x, c.rect.y, c.rect.w, splitY)
		l.buildLayout(node.First, Canvas{c.screen, r1, c.grid})

		r2 := NewRect(c.rect.x, splitY+1, c.rect.w, c.rect.h-splitY-1)
		l.buildLayout(node.Second, Canvas{c.screen, r2, c.grid})

	}
}
