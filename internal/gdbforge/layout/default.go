package layout

import (
	"github.com/yairgd/termforge"
	"github.com/yairgd/termforge/platform"
)

// DefaultSpec builds the multi-pane default workspace.
// Ratios come from AppState.DefaultLayoutRatios (Left, Output, BottomFirst).
type DefaultSpec struct {
	Ratios platform.DefaultLayoutRatios
}

func (s DefaultSpec) Name() string { return Default }

func (s DefaultSpec) Build(panes Panes) *termforge.WidgetTree {
	return BuildDefault(panes, s.Ratios)
}

// BuildDefault builds:
//
//	Vertical: left = Code over GDB; right = Output / Breakpoints / Threads / Call stack.
func BuildDefault(panes Panes, ratios platform.DefaultLayoutRatios) *termforge.WidgetTree {
	ratios.Left = clampRatio(ratios.Left)
	ratios.Output = clampRatio(ratios.Output)
	ratios.BottomFirst = clampRatio(ratios.BottomFirst)
	tree := termforge.NewWidgetTree(panes.Code)
	tree.SetEqualAlways(true)
	tree.Split(termforge.Vertical, panes.Output)
	tree.FocusWidget(panes.Code)
	tree.Split(termforge.Horizontal, panes.GDB)
	tree.FocusWidget(panes.Output)
	tree.Split(termforge.Horizontal, panes.Breakpoints)
	tree.FocusWidget(panes.Breakpoints)
	tree.Split(termforge.Horizontal, panes.Threads)
	tree.FocusWidget(panes.Threads)
	tree.Split(termforge.Horizontal, panes.Callstack)
	tree.FocusWidget(panes.GDB)
	applyDefaultRatios(tree.Root(), ratios)
	tree.SetEqualAlways(false)
	return tree
}

func applyDefaultRatios(root *termforge.Node, ratios platform.DefaultLayoutRatios) {
	if root == nil || root.Type != termforge.NodeSplit || root.Dir != termforge.Vertical {
		return
	}
	root.Ratio = ratios.Left
	right := root.Second
	if right == nil || right.Type != termforge.NodeSplit || right.Dir != termforge.Horizontal {
		return
	}
	right.Ratio = ratios.Output
	bottom := right.Second
	if bottom == nil || bottom.Type != termforge.NodeSplit || bottom.Dir != termforge.Horizontal {
		return
	}
	bottom.Ratio = ratios.BottomFirst
	rest := bottom.Second
	if rest != nil && rest.Type == termforge.NodeSplit && rest.Dir == termforge.Horizontal {
		rest.Ratio = 0.5
	}
}
