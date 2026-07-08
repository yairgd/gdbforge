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

func (n *Node) ComputeLeaves() int {
	if n == nil {
		return 0
	}

	if n.Type == NodeLeaf {
		n.Leaves = 1
		return 1
	}

	left := n.First.ComputeLeaves()
	right := n.Second.ComputeLeaves()

	n.Leaves = left + right

	return n.Leaves
}

func (n *Node) ComputeRatios() {
	if n == nil || n.Type == NodeLeaf {
		return
	}

	// Weight by leaves along this split's direction only, so a perpendicular
	// nested split counts as a single unit. This keeps outer proportions stable
	// when panes are added in the opposite direction (matches the pre-drag
	// Units()-based sizing).
	first := Units(n.First, n.Dir)
	total := Units(n, n.Dir)

	n.Ratio = float64(first) / float64(total)

	n.First.ComputeRatios()
	n.Second.ComputeRatios()
}

func (n *Node) Rebalance(dir SplitDir) int {
	if n == nil {
		return 0
	}

	if n.Type == NodeLeaf {
		return 1
	}

	// Different split direction -> treat subtree as one unit
	if n.Dir != dir {
		return 1
	}

	left := n.First.Rebalance(dir)
	right := n.Second.Rebalance(dir)

	n.Ratio = float64(left) / float64(left+right)

	return left + right
}

func Units(n *Node, dir SplitDir) int {
	if n == nil {
		return 0
	}

	if n.Type == NodeLeaf {
		return 1
	}

	if n.Dir == dir {
		return Units(n.First, dir) +
			Units(n.Second, dir)
	}

	// continue through opposite split
	return max(
		Units(n.First, dir),
		Units(n.Second, dir),
	)
}
func (n *Node) TotLeaves(out *[]*Node) {
	if n == nil {
		return
	}

	if n.Type == NodeLeaf {
		*out = append(*out, n)
		return
	}

	n.First.TotLeaves(out)
	n.Second.TotLeaves(out)
}

type Node struct {
	Type NodeType // Defines whether this is a Leaf or Split node

	// --- Leaf node fields ---
	Widget Widget // The UI component stored in this node (only valid if Type == Leaf)
	//Rect   Rect

	canvas Canvas

	// --- Split node fields ---
	Dir   SplitDir // Direction of the split (Horizontal or Vertical)
	Ratio float64  // Portion of space given to the First child (range: 0.0–1.0)

	First  *Node // First child node (top or left depending on Dir)
	Second *Node // Second child node (bottom or right depending on Dir)
	Leaves int

	parent *Node

	// Separator geometry (screen coords, valid after BuildLayout on split nodes).
	layoutRect Rect
	sepRect    Rect
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
	//n.Rect = r
}

//func (n *Node) Draw(r Rect) {
//	if n.Type == NodeLeaf {
//		n.Widget.Draw(r)
//		return
//	}
//	n.First.Draw(n.First.Rect)
//	n.Second.Draw(n.Second.Rect)
//}

func (n *Node) Draw(c Canvas) {
	if n.Type == NodeLeaf {
		n.Widget.Draw(c)
		return
	}
	n.First.Draw(n.First.canvas)
	n.Second.Draw(n.Second.canvas)
}

func (n *Node) Draw_org(r Rect) {
	if n.Type == NodeLeaf {
		//n.Widget.Draw(r)
		return
	}

	if n.Dir == Vertical {
		total := Units(n, Vertical)

		w1 := r.w * Units(n.First, Vertical) / total
		w2 := r.w - w1

		n.First.Draw_org(NewRect(r.X(), r.Y(), w1, r.H()))
		n.Second.Draw_org(NewRect(r.X()+w1, r.Y(), w2, r.H()))

	} else {
		total := Units(n, Horizontal)

		h1 := r.h * Units(n.First, Horizontal) / total
		h2 := r.h - h1

		n.First.Draw_org(NewRect(r.X(), r.Y(), r.W(), h1))
		n.Second.Draw_org(NewRect(r.X(), r.Y()+h1, r.W(), h2))
	}
}
