package termui

// layout_tree.go holds binary-tree traversal and ratio algorithms for Node.
// It is UI-free: no Canvas/Rect dependencies. WidgetTree supplies extents when
// pin helpers need absolute sizes from the last layout frame.

// WalkLeaves visits every leaf under n in left-to-right order.
func WalkLeaves(n *Node, fn func(*Node)) {
	if n == nil {
		return
	}
	if n.Type == NodeLeaf {
		fn(n)
		return
	}
	WalkLeaves(n.First, fn)
	WalkLeaves(n.Second, fn)
}

// WalkSplits visits every split under n in pre-order (parent before children).
func WalkSplits(n *Node, fn func(*Node)) {
	if n == nil {
		return
	}
	if n.Type != NodeSplit {
		return
	}
	fn(n)
	WalkSplits(n.First, fn)
	WalkSplits(n.Second, fn)
}

// WalkPreOrder visits every node. If fn returns false, children are skipped.
func WalkPreOrder(n *Node, fn func(*Node) bool) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	if n.Type == NodeSplit {
		WalkPreOrder(n.First, fn)
		WalkPreOrder(n.Second, fn)
	}
}

// WalkPostOrder visits every node after its children.
func WalkPostOrder(n *Node, fn func(*Node)) {
	if n == nil {
		return
	}
	if n.Type == NodeSplit {
		WalkPostOrder(n.First, fn)
		WalkPostOrder(n.Second, fn)
	}
	fn(n)
}

// CollectLeaves returns all leaf nodes under n in left-to-right order.
func CollectLeaves(n *Node) []*Node {
	var leaves []*Node
	WalkLeaves(n, func(leaf *Node) {
		leaves = append(leaves, leaf)
	})
	return leaves
}

// Units returns the directional weight of subtree n along dir.
// Same-direction splits sum their children; opposite-direction splits
// contribute the max of their children (treated as a single unit along dir).
func Units(n *Node, dir SplitDir) int {
	if n == nil {
		return 0
	}
	if n.Type == NodeLeaf {
		return 1
	}
	if n.Dir == dir {
		return Units(n.First, dir) + Units(n.Second, dir)
	}
	return max(Units(n.First, dir), Units(n.Second, dir))
}

// ComputeRatios sets each split's Ratio from directional leaf weights so panes
// are evenly sized (1/N per direction).
func ComputeRatios(n *Node) {
	if n == nil || n.Type == NodeLeaf {
		return
	}
	// Weight by leaves along this split's direction only, so a perpendicular
	// nested split counts as a single unit. This keeps outer proportions stable
	// when panes are added in the opposite direction.
	first := Units(n.First, n.Dir)
	total := Units(n, n.Dir)
	n.Ratio = float64(first) / float64(total)
	ComputeRatios(n.First)
	ComputeRatios(n.Second)
}

// pinFirstEdge walks the far (right/bottom) edge of a subtree that will occupy
// region cells along dir, keeping every First child at its current absolute
// size so only the edge-most leaf absorbs the change.
// extent returns the node's size along dir from the caller's layout frame.
func pinFirstEdge(node *Node, dir SplitDir, region int, extent func(*Node) int) {
	for node != nil && node.Type == NodeSplit && node.Dir == dir {
		avail := region - 1
		if avail < minPaneCells*2 {
			return
		}
		firstAbs := extent(node.First)
		if firstAbs < minPaneCells {
			firstAbs = minPaneCells
		}
		if firstAbs > avail-minPaneCells {
			firstAbs = avail - minPaneCells
		}
		node.Ratio = float64(firstAbs) / float64(avail)
		region = region - firstAbs - 1
		node = node.Second
	}
}

// pinSecondEdge is the mirror of pinFirstEdge for the near (left/top) edge:
// it keeps every Second child at its current absolute size.
func pinSecondEdge(node *Node, dir SplitDir, region int, extent func(*Node) int) {
	for node != nil && node.Type == NodeSplit && node.Dir == dir {
		avail := region - 1
		if avail < minPaneCells*2 {
			return
		}
		secondAbs := extent(node.Second)
		if secondAbs < minPaneCells {
			secondAbs = minPaneCells
		}
		if secondAbs > avail-minPaneCells {
			secondAbs = avail - minPaneCells
		}
		firstW := avail - secondAbs
		node.Ratio = float64(firstW) / float64(avail)
		region = firstW
		node = node.First
	}
}
