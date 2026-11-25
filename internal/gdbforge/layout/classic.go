package layout

import "github.com/yairgd/termforge"

// BuildClassic builds a single horizontal split: Code over GDB.
func BuildClassic(panes Panes) *termforge.WidgetTree {
	tree := termforge.NewWidgetTree(panes.Code)
	tree.SetEqualAlways(true)
	tree.Split(termforge.Horizontal, panes.GDB)
	tree.FocusWidget(panes.GDB)
	// Classic source-heavy default: Code gets 2/3 height.
	if root := tree.Root(); root != nil && root.Type == termforge.NodeSplit {
		root.Ratio = 2.0 / 3.0
	}
	tree.SetEqualAlways(false)
	return tree
}
