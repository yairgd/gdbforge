package termui

import (
	"github.com/gdamore/tcell/v2"
)

func split(node *Node, dir SplitDir, newWidget Widget, oldWidget Widget) {
	var old Widget
	if oldWidget == nil {
		old = node.Widget
	} else {
		old = oldWidget
	}

	node.Type = NodeSplit
	node.Dir = dir
	node.Ratio = 0.5

	node.First = &Node{Type: NodeLeaf, Widget: old}
	node.Second = &Node{Type: NodeLeaf, Widget: newWidget}

	node.Widget = nil
}

type Layout struct {
	BaseWidget
	Root  *Node
	Focus *Node
}

func (l *Layout) NewSplit(dir SplitDir, w Widget) {
	split(l.Focus, dir, w, nil)
}

func (l *Layout) HandleEvent(ev tcell.Event) {
	l.Root.HandleEvent(ev)
}

func (l *Layout) SetSize(w, h int) {
	l.Focus.SetSize(w, h)
}

func (l *Layout) Draw(r Rect) {
	l.Root.Draw(r)
}

func (l *Layout) ActiveWidget() *Widget {
	return &l.Focus.First.Widget
}

func NewLayout(root *Node, uiContext UIContext) *Layout {
	return &Layout{
		BaseWidget: NewBaseWidget(uiContext),
		Root:       root,
		Focus:      root,
	}
}
