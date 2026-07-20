package layout

import (
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// DefaultSpec builds the multi-pane default workspace.
// Ratios come from AppState.DefaultLayoutRatios (Left, Output, BottomFirst).
type DefaultSpec struct {
	Ratios platform.DefaultLayoutRatios
}

func (s DefaultSpec) Name() string { return Default }

func (s DefaultSpec) Build(title string, panes Panes) *termui.TabWidget {
	return BuildDefault(title, panes, s.Ratios)
}

// BuildDefault builds:
//
//	Vertical: left = Code over GDB; right = Output / Breakpoints / Threads / Call stack.
func BuildDefault(title string, panes Panes, ratios platform.DefaultLayoutRatios) *termui.TabWidget {
	ratios.Left = clampRatio(ratios.Left)
	ratios.Output = clampRatio(ratios.Output)
	ratios.BottomFirst = clampRatio(ratios.BottomFirst)
	tree := termui.NewWidgetTree(panes.Code)
	tree.SetEqualAlways(true)
	tree.Split(termui.Vertical, panes.Output)
	tree.FocusWidget(panes.Code)
	tree.Split(termui.Horizontal, panes.GDB)
	tree.FocusWidget(panes.Output)
	tree.Split(termui.Horizontal, panes.Breakpoints)
	tree.FocusWidget(panes.Breakpoints)
	tree.Split(termui.Horizontal, panes.Threads)
	tree.FocusWidget(panes.Threads)
	tree.Split(termui.Horizontal, panes.Callstack)
	tree.FocusWidget(panes.GDB)
	applyDefaultRatios(tree.Root(), ratios)
	tree.SetEqualAlways(false)
	return termui.NewTabWidget(title, tree)
}

func applyDefaultRatios(root *termui.Node, ratios platform.DefaultLayoutRatios) {
	if root == nil || root.Type != termui.NodeSplit || root.Dir != termui.Vertical {
		return
	}
	root.Ratio = ratios.Left
	right := root.Second
	if right == nil || right.Type != termui.NodeSplit || right.Dir != termui.Horizontal {
		return
	}
	right.Ratio = ratios.Output
	bottom := right.Second
	if bottom == nil || bottom.Type != termui.NodeSplit || bottom.Dir != termui.Horizontal {
		return
	}
	bottom.Ratio = ratios.BottomFirst
	rest := bottom.Second
	if rest != nil && rest.Type == termui.NodeSplit && rest.Dir == termui.Horizontal {
		rest.Ratio = 0.5
	}
}
