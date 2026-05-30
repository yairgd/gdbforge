package termui

import (
	"github.com/gdamore/tcell/v2"
)

type NodeType int

const (
	NodeLeaf  NodeType = iota // A leaf node: contains a single widget (no children)
	NodeSplit                 // A split node: divides space between two child nodes
)

type SplitDir int

const (
	Horizontal SplitDir = iota // Split top/bottom (one above the other)
	Vertical                   // Split left/right (side by side)
)

func Units(n *Node, dir SplitDir) int {
	if n.Type == NodeLeaf {
		return 1
	}

	if n.Dir == dir {
		return Units(n.First, dir) + Units(n.Second, dir)
	}

	return 1
}

type Node struct {
	Type NodeType // Defines whether this is a Leaf or Split node

	// --- Leaf node fields ---
	Widget Widget // The UI component stored in this node (only valid if Type == Leaf)
	Rect   Rect

	// --- Split node fields ---
	Dir   SplitDir // Direction of the split (Horizontal or Vertical)
	Ratio float64  // Portion of space given to the First child (range: 0.0–1.0)

	First  *Node // First child node (top or left depending on Dir)
	Second *Node // Second child node (bottom or right depending on Dir)
}

func (n *Node) HandleEvent(ev tcell.Event) {
	if n.Type == NodeLeaf {
		n.Widget.HandleEvent(ev)
	} else {
		n.First.Widget.HandleEvent(ev)
		n.Second.Widget.HandleEvent(ev)
	}

}

func (n *Node) SetRect(r Rect) {
	n.Rect = r
}

func (n *Node) Draw(r Rect) {
	if n.Type == NodeLeaf {
		n.Widget.Draw(r)
		return
	}
	n.First.Draw(n.First.Rect)
	n.Second.Draw(n.Second.Rect)
}

func (n *Node) Draw_org(r Rect) {
	if n.Type == NodeLeaf {
		n.Widget.Draw(r)
		return
	}

	if n.Dir == Vertical {
		total := Units(n, Vertical)

		w1 := r.w * Units(n.First, Vertical) / total
		w2 := r.w - w1

		n.First.Draw(Rect{r.x, r.y, w1, r.h})
		n.Second.Draw(Rect{r.x + w1, r.y, w2, r.h})

	} else {
		total := Units(n, Horizontal)

		h1 := r.h * Units(n.First, Horizontal) / total
		h2 := r.h - h1

		n.First.Draw(Rect{r.x, r.y, r.w, h1})
		n.Second.Draw(Rect{r.x, r.y + h1, r.w, h2})
	}
}
