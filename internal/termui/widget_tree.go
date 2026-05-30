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

func (l *WidgetTree) Draw() {
	l.draw(l.root)
}

func (l *WidgetTree) draw(node *Node) {
	if node.Type == NodeLeaf {
		node.Widget.Draw(node.canvas)
		return
	}
	l.draw(node.First)
	l.draw(node.Second)

}

func (l *WidgetTree) BuildLayout(
	rect Rect,
	grid *Grid) {
	l.buildLayout(l.root, rect, grid)

}

func (l *WidgetTree) buildLayout(
	node *Node,
	rect Rect,
	grid *Grid,
) {

	if node == nil {
		return
	}

	if node.Type == NodeLeaf {
		node.canvas = Canvas{rect, grid}
		node.SetRect(rect)

		return
	}

	switch node.Dir {

	case Vertical:

		leftW := int(float64(rect.w) * node.Ratio)

		splitX := rect.x + leftW

		/*
			Draw separator line.
		*/
		grid.DrawVertical(
			splitX,
			rect.y-1,
			rect.y+rect.h+1,
			false,
		)

		r1 := NewRect(rect.x, rect.y, splitX-0, rect.h)
		l.buildLayout(node.First, r1, grid)

		r2 := NewRect(splitX+1, rect.y, rect.w-splitX-1, rect.h)
		l.buildLayout(node.Second, r2, grid)

	case Horizontal:

		topH := int(float64(rect.h) * node.Ratio)

		splitY := rect.y + topH

		grid.DrawHorizontal(
			splitY,
			rect.x-1,
			rect.x+rect.w+1,
			false,
		)

		r1 := NewRect(rect.x, rect.y, rect.w, splitY)
		l.buildLayout(node.First, r1, grid)

		r2 := NewRect(rect.x, splitY+1, rect.w, rect.h-splitY-1)
		l.buildLayout(node.Second, r2, grid)

	}
}
