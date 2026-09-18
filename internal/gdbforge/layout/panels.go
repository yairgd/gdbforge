package layout

import "github.com/yairgd/termforge"

// Panels ratios: left Code+GDB 2/3; right Output 1/2 over bottom half;
// bottom half = (Threads | Callstack) 2/3 over Breakpoints 1/3.
const (
	panelsLeftRatio             = 2.0 / 3.0
	panelsOutputRatio           = 1.0 / 2.0
	panelsThreadsCallstackRatio = 2.0 / 3.0
	panelsThreadsRatio          = 1.0 / 2.0 // Threads | Callstack share equally
)

// PanelsSpec builds the Output + Threads|Callstack / Breakpoints side layout.
type PanelsSpec struct{}

func (PanelsSpec) Name() string { return Panels }

func (PanelsSpec) Build(panes Panes) *termforge.WidgetTree {
	return BuildPanels(panes)
}

// BuildPanels builds:
//
//	Vertical: left = Code over GDB; right = Output over bottom half.
//	Bottom half: (Threads | Callstack) over Breakpoints — Threads left, Callstack
//	right, taking 2/3 of the bottom half; Breakpoints the remaining 1/3.
func BuildPanels(panes Panes) *termforge.WidgetTree {
	tree := termforge.NewWidgetTree(panes.Code)
	tree.SetEqualAlways(true)
	tree.Split(termforge.Vertical, panes.Output)
	tree.FocusWidget(panes.Code)
	tree.Split(termforge.Horizontal, panes.GDB)
	tree.FocusWidget(panes.Output)
	tree.Split(termforge.Horizontal, panes.Threads)
	tree.FocusWidget(panes.Threads)
	tree.Split(termforge.Horizontal, panes.Breakpoints)
	tree.FocusWidget(panes.Threads)
	tree.Split(termforge.Vertical, panes.Callstack)
	tree.FocusWidget(panes.GDB)
	applyPanelsRatios(tree.Root())
	tree.SetEqualAlways(false)
	return tree
}

func applyPanelsRatios(root *termforge.Node) {
	if root == nil || root.Type != termforge.NodeSplit || root.Dir != termforge.Vertical {
		return
	}
	root.Ratio = panelsLeftRatio
	right := root.Second
	if right == nil || right.Type != termforge.NodeSplit || right.Dir != termforge.Horizontal {
		return
	}
	right.Ratio = panelsOutputRatio
	bottom := right.Second // (Threads|Callstack) over Breakpoints
	if bottom == nil || bottom.Type != termforge.NodeSplit || bottom.Dir != termforge.Horizontal {
		return
	}
	bottom.Ratio = panelsThreadsCallstackRatio
	pair := bottom.First // Threads | Callstack
	if pair != nil && pair.Type == termforge.NodeSplit && pair.Dir == termforge.Vertical {
		pair.Ratio = panelsThreadsRatio
	}
}
