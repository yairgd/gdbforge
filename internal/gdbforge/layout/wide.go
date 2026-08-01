package layout

import "github.com/yairgd/gdbforge/internal/termui"

// Wide ratios: top Code|IO half height; bottom GDB | side column;
// side = (Threads|Callstack) 2/3 over Breakpoints 1/3.
const (
	wideTopRatio              = 1.0 / 2.0
	wideCodeRatio             = 1.0 / 2.0 // Code | Output
	wideGdbRatio              = 1.0 / 2.0 // GDB | side
	wideThreadsCallstackRatio = 2.0 / 3.0
	wideThreadsRatio          = 1.0 / 2.0 // Threads | Callstack
)

// WideSpec builds Code|IO over GDB|(Threads|Callstack / Breakpoints).
type WideSpec struct{}

func (WideSpec) Name() string { return Wide }

func (WideSpec) Build(panes Panes) *termui.WidgetTree {
	return BuildWide(panes)
}

// BuildWide builds:
//
//	Horizontal: top = Code | Output; bottom = GDB | side.
//	Side: (Threads | Callstack) over Breakpoints — top pair 2/3, BP 1/3.
func BuildWide(panes Panes) *termui.WidgetTree {
	tree := termui.NewWidgetTree(panes.Code)
	tree.SetEqualAlways(true)
	tree.Split(termui.Horizontal, panes.GDB)
	tree.FocusWidget(panes.Code)
	tree.Split(termui.Vertical, panes.Output)
	tree.FocusWidget(panes.GDB)
	tree.Split(termui.Vertical, panes.Threads)
	tree.FocusWidget(panes.Threads)
	tree.Split(termui.Horizontal, panes.Breakpoints)
	tree.FocusWidget(panes.Threads)
	tree.Split(termui.Vertical, panes.Callstack)
	tree.FocusWidget(panes.GDB)
	applyWideRatios(tree.Root())
	tree.SetEqualAlways(false)
	return tree
}

func applyWideRatios(root *termui.Node) {
	if root == nil || root.Type != termui.NodeSplit || root.Dir != termui.Horizontal {
		return
	}
	root.Ratio = wideTopRatio

	top := root.First // Code | Output
	if top != nil && top.Type == termui.NodeSplit && top.Dir == termui.Vertical {
		top.Ratio = wideCodeRatio
	}

	bottom := root.Second // GDB | side
	if bottom == nil || bottom.Type != termui.NodeSplit || bottom.Dir != termui.Vertical {
		return
	}
	bottom.Ratio = wideGdbRatio

	side := bottom.Second // (Threads|Callstack) over Breakpoints
	if side == nil || side.Type != termui.NodeSplit || side.Dir != termui.Horizontal {
		return
	}
	side.Ratio = wideThreadsCallstackRatio
	pair := side.First
	if pair != nil && pair.Type == termui.NodeSplit && pair.Dir == termui.Vertical {
		pair.Ratio = wideThreadsRatio
	}
}
