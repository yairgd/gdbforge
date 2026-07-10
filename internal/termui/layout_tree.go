package termui

// layout_tree.go holds binary-tree traversal and layout algorithms for Node.
// Node itself is a pure data object; WidgetTree expresses GUI behavior via
// these visitors rather than implementing its own recursion.

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

// WalkWithContext walks the tree while threading a per-node context value.
// onLeaf is called for leaves. onSplit is called for splits and must return
// the contexts for First and Second; if cont is false, children are skipped.
func WalkWithContext[T any](
	n *Node,
	ctx T,
	onLeaf func(n *Node, ctx T),
	onSplit func(n *Node, ctx T) (first, second T, cont bool),
) {
	if n == nil {
		return
	}
	if n.Type == NodeLeaf {
		if onLeaf != nil {
			onLeaf(n, ctx)
		}
		return
	}
	var first, second T
	cont := true
	if onSplit != nil {
		first, second, cont = onSplit(n, ctx)
	} else {
		first, second = ctx, ctx
	}
	if !cont {
		return
	}
	WalkWithContext(n.First, first, onLeaf, onSplit)
	WalkWithContext(n.Second, second, onLeaf, onSplit)
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

// nodeExtent returns the node's current size (in cells) along dir, as of the
// last BuildLayout. Leaves store their region in canvas; splits in layoutRect.
func nodeExtent(n *Node, dir SplitDir) int {
	var r Rect
	if n.Type == NodeLeaf {
		r = n.canvas.Rect()
	} else {
		r = n.layoutRect
	}
	if dir == Vertical {
		return r.W()
	}
	return r.H()
}

// pinFirstEdge walks the far (right/bottom) edge of a subtree that will occupy
// region cells along dir, keeping every First child at its current absolute
// size so only the edge-most leaf absorbs the change.
func pinFirstEdge(node *Node, dir SplitDir, region int) {
	for node != nil && node.Type == NodeSplit && node.Dir == dir {
		avail := region - 1
		if avail < minPaneCells*2 {
			return
		}
		firstAbs := nodeExtent(node.First, dir)
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
func pinSecondEdge(node *Node, dir SplitDir, region int) {
	for node != nil && node.Type == NodeSplit && node.Dir == dir {
		avail := region - 1
		if avail < minPaneCells*2 {
			return
		}
		secondAbs := nodeExtent(node.Second, dir)
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
