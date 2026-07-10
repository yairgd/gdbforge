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

// Node is a pure data object for the binary layout tree.
// Tree algorithms live in layout_tree.go; GUI behavior lives on WidgetTree.
type Node struct {
	Type NodeType // Defines whether this is a Leaf or Split node

	// --- Leaf node fields ---
	Widget Widget // The UI component stored in this node (only valid if Type == Leaf)
	canvas Canvas

	// --- Split node fields ---
	Dir   SplitDir // Direction of the split (Horizontal or Vertical)
	Ratio float64  // Portion of space given to the First child (range: 0.0–1.0)

	First  *Node // First child node (top or left depending on Dir)
	Second *Node // Second child node (bottom or right depending on Dir)

	parent *Node

	// Separator geometry (screen coords, valid after BuildLayout on split nodes).
	layoutRect Rect
	sepRect    Rect
}
