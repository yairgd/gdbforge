// Package demo holds panes/layout for the host showcase binary (cmd/demo).
// It must not import gdb, dlv, mcp, or gdbforge.
package demo

import "github.com/yairgd/gdbforge/internal/termui"

// Panes are the three stub views in the default demo layout.
type Panes struct {
	Main termui.Widget // primary console
	Side termui.Widget // status / notes
	Log  termui.Widget // scrollable log
}

// BuildDefault builds Main|Side over Log (gdbforge-like multi-pane chrome).
func BuildDefault(title string, p Panes) *termui.TabWidget {
	tree := termui.NewWidgetTree(p.Main)
	tree.SetEqualAlways(true)
	tree.Split(termui.Horizontal, p.Log)
	tree.FocusWidget(p.Main)
	tree.Split(termui.Vertical, p.Side)
	if root := tree.Root(); root != nil && root.Type == termui.NodeSplit {
		root.Ratio = 2.0 / 3.0 // top band taller
		if root.First != nil && root.First.Type == termui.NodeSplit {
			root.First.Ratio = 2.0 / 3.0 // Main wider than Side
		}
	}
	tree.SetEqualAlways(false)
	tree.FocusWidget(p.Main)
	return termui.NewTabWidget(title, tree)
}

// HelpText is shown by :help in the main pane.
const HelpText = `demo — host showcase (no debugger)

Same TUI chrome as gdbforge: multi-pane layout, : cmdline, focus splits.

Commands:
  :window left|right|up|down
  :vs / :split     split focused pane
  :b main|side|log|help
  :clear           clear focused pane
  :help            this text
  :quit            close pane / exit

Keys (Normal):
  :                command line
  Esc              normal mode
  Ctrl-W h/j/k/l   focus
  Ctrl-D           quit
`
