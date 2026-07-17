package termui

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

// Node is structural tree state only. Per-frame paint/hit geometry lives in
// WidgetTree.geom; tree algorithms live in layout_tree.go.
type Node struct {
	Type NodeType // Defines whether this is a Leaf or Split node

	// --- Leaf node fields ---
	Widget Widget // The UI component stored in this node (only valid if Type == Leaf)

	// --- Split node fields ---
	Dir   SplitDir // Direction of the split (Horizontal or Vertical)
	Ratio float64  // Portion of space given to the First child (range: 0.0–1.0)

	First  *Node // First child node (top or left depending on Dir)
	Second *Node // Second child node (bottom or right depending on Dir)

	parent *Node
}

// SetWidget replaces the widget on a leaf node in O(1). Layout/geometry are unchanged.
// No-op if n is nil or not a leaf.
func (n *Node) SetWidget(w Widget) {
	if n == nil || n.Type != NodeLeaf {
		return
	}
	n.Widget = w
}

// GetWidget returns the widget on this node (may be nil).
func (n *Node) GetWidget() Widget {
	if n == nil {
		return nil
	}
	return n.Widget
}
