package main

import (
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

// newTabDefaultDebugLayout builds the default debugger workspace:
//
//	Vertical: left = Code over GDB; right = Output / Breakpoints / Threads / Call stack.
//	ratios come from AppState.DefaultLayoutRatios (Left, Output, BottomFirst).
func newTabDefaultDebugLayout(title string, code, gdb, output, bp, threads, callstack termui.Widget, ratios platform.DefaultLayoutRatios) *termui.TabWidget {
	ratios.Left = clampLayoutRatio(ratios.Left)
	ratios.Output = clampLayoutRatio(ratios.Output)
	ratios.BottomFirst = clampLayoutRatio(ratios.BottomFirst)
	tree := termui.NewWidgetTree(code)
	// Temporarily equalize while building the tree so nested panes get even shares.
	tree.SetEqualAlways(true)
	tree.Split(termui.Vertical, output)
	tree.FocusWidget(code)
	tree.Split(termui.Horizontal, gdb)
	tree.FocusWidget(output)
	tree.Split(termui.Horizontal, bp)
	tree.FocusWidget(bp)
	tree.Split(termui.Horizontal, threads)
	tree.FocusWidget(threads)
	tree.Split(termui.Horizontal, callstack)
	tree.FocusWidget(gdb)
	applyDefaultDebugRatios(tree.Root(), ratios)
	// Caller enables equalalways as policy without wiping the preset ratios.
	tree.SetEqualAlways(false)
	return termui.NewTabWidget(title, tree)
}

func clampLayoutRatio(r float64) float64 {
	if r < 0.1 {
		return 0.1
	}
	if r > 0.9 {
		return 0.9
	}
	return r
}

// applyDefaultDebugRatios sets outer left/right width and right-column heights
// from DefaultLayoutRatios (Output share; BottomFirst for Breakpoints in the
// bottom half; Threads|Call stack split the remainder of that half).
func applyDefaultDebugRatios(root *termui.Node, ratios platform.DefaultLayoutRatios) {
	if root == nil || root.Type != termui.NodeSplit || root.Dir != termui.Vertical {
		return
	}
	root.Ratio = ratios.Left
	right := root.Second // Output over (BP / Threads / Call stack)
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
		rest.Ratio = 0.5 // Threads | Call stack share the remaining bottom equally
	}
}
