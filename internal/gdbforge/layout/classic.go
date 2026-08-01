package layout

import "github.com/yairgd/gdbforge/internal/termui"

// ClassicSpec builds the original cgdb view: Code over GDB, full width.
type ClassicSpec struct{}

func (ClassicSpec) Name() string { return Classic }

func (ClassicSpec) Build(panes Panes) *termui.WidgetTree {
	return BuildClassic(panes)
}

// BuildClassic builds a single horizontal split: Code over GDB.
func BuildClassic(panes Panes) *termui.WidgetTree {
	tree := termui.NewWidgetTree(panes.Code)
	tree.SetEqualAlways(true)
	tree.Split(termui.Horizontal, panes.GDB)
	tree.FocusWidget(panes.GDB)
	// Classic source-heavy default: Code gets 2/3 height.
	if root := tree.Root(); root != nil && root.Type == termui.NodeSplit {
		root.Ratio = 2.0 / 3.0
	}
	tree.SetEqualAlways(false)
	return tree
}
