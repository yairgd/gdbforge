package layout

import "github.com/yairgd/gdbforge/internal/termui"

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

func (PanelsSpec) Build(title string, panes Panes) *termui.TabWidget {
	return BuildPanels(title, panes)
}

// BuildPanels builds:
//
//	Vertical: left = Code over GDB; right = Output over bottom half.
//	Bottom half: (Threads | Callstack) over Breakpoints — Threads left, Callstack
//	right, taking 2/3 of the bottom half; Breakpoints the remaining 1/3.
func BuildPanels(title string, panes Panes) *termui.TabWidget {
	tree := termui.NewWidgetTree(panes.Code)
	tree.SetEqualAlways(true)
	tree.Split(termui.Vertical, panes.Output)
	tree.FocusWidget(panes.Code)
	tree.Split(termui.Horizontal, panes.GDB)
	tree.FocusWidget(panes.Output)
	tree.Split(termui.Horizontal, panes.Threads)
	tree.FocusWidget(panes.Threads)
	tree.Split(termui.Horizontal, panes.Breakpoints)
	tree.FocusWidget(panes.Threads)
	tree.Split(termui.Vertical, panes.Callstack)
	tree.FocusWidget(panes.GDB)
	applyPanelsRatios(tree.Root())
	tree.SetEqualAlways(false)
	return termui.NewTabWidget(title, tree)
}

func applyPanelsRatios(root *termui.Node) {
	if root == nil || root.Type != termui.NodeSplit || root.Dir != termui.Vertical {
		return
	}
	root.Ratio = panelsLeftRatio
	right := root.Second
	if right == nil || right.Type != termui.NodeSplit || right.Dir != termui.Horizontal {
		return
	}
	right.Ratio = panelsOutputRatio
	bottom := right.Second // (Threads|Callstack) over Breakpoints
	if bottom == nil || bottom.Type != termui.NodeSplit || bottom.Dir != termui.Horizontal {
		return
	}
	bottom.Ratio = panelsThreadsCallstackRatio
	pair := bottom.First // Threads | Callstack
	if pair != nil && pair.Type == termui.NodeSplit && pair.Dir == termui.Vertical {
		pair.Ratio = panelsThreadsRatio
	}
}
