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

func (n *Node) SetSize(w, h int) {
}

func (n *Node) Draw(x, y, w, h int) {
	if n.Type == NodeLeaf {
		n.Widget.Draw(x, y, w, h)
		return
	}

	if n.Dir == Vertical {
		total := Units(n, Vertical)

		w1 := w * Units(n.First, Vertical) / total
		w2 := w - w1

		n.First.Draw(x, y, w1, h)
		n.Second.Draw(x+w1, y, w2, h)

	} else {
		total := Units(n, Horizontal)

		h1 := h * Units(n.First, Horizontal) / total
		h2 := h - h1

		n.First.Draw(x, y, w, h1)
		n.Second.Draw(x, y+h1, w, h2)
	}
}
