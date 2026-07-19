package main

import (
	"context"
	"time"

	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

// Named leaf marks on the active WidgetTree (role names live only in cmd/cgdb).
const (
	leafMarkCode = "code"
	leafMarkGDB  = "gdb"
)

// activeCodeWidget returns the CodeWidget buffer Esc / global keys should drive.
func (a *DebuggerApp) activeCodeWidget() *widgets.CodeWidget {
	if a.tab != nil {
		if cw, ok := a.tab.FocusedWidget().(*widgets.CodeWidget); ok && cw != nil {
			return cw
		}
	}
	if path := a.State().CurrentFile(); path != "" {
		if w, _ := a.ensureCodeBuffer(path); w != nil {
			return w
		}
	}
	if a.primaryCode != nil {
		return a.primaryCode
	}
	for _, w := range a.fileBuffers {
		if w != nil {
			return w
		}
	}
	return a.layoutCodeWidget()
}

func isCodeWidget(w termui.Widget) bool {
	_, ok := w.(*widgets.CodeWidget)
	return ok
}

// findCodeLeaf returns the remembered code leaf if still valid, else any leaf
// currently showing a CodeWidget.
func (a *DebuggerApp) findCodeLeaf() *termui.Node {
	if a.tab == nil {
		return nil
	}
	if leaf := a.tab.LeafMark(leafMarkCode); leaf != nil && isCodeWidget(leaf.GetWidget()) {
		return leaf
	}
	leaf := a.tab.FindLeaf(isCodeWidget)
	a.tab.SetLeafMark(leafMarkCode, leaf)
	return leaf
}

// rememberCodeLeafFromFocus stores the focused leaf when it shows a CodeWidget.
func (a *DebuggerApp) rememberCodeLeafFromFocus() {
	if a.tab == nil {
		return
	}
	tree := a.tab.ActiveTree()
	if tree == nil {
		return
	}
	leaf := tree.FocusedLeaf()
	if leaf == nil {
		return
	}
	if isCodeWidget(leaf.GetWidget()) {
		a.tab.SetLeafMark(leafMarkCode, leaf)
	}
	if leaf.GetWidget() == a.gdbWidget {
		a.tab.SetLeafMark(leafMarkGDB, leaf)
	}
}

// findGdbLeaf returns the remembered GDB leaf if it still shows GDB, else any
// leaf currently showing the GDBWidget.
func (a *DebuggerApp) findGdbLeaf() *termui.Node {
	if a.tab == nil || a.gdbWidget == nil {
		return nil
	}
	if leaf := a.tab.LeafMark(leafMarkGDB); leaf != nil && leaf.GetWidget() == a.gdbWidget {
		return leaf
	}
	leaf := a.tab.FindLeaf(func(w termui.Widget) bool { return w == a.gdbWidget })
	a.tab.SetLeafMark(leafMarkGDB, leaf)
	return leaf
}

// pickGdbFallbackLeaf chooses a leaf to host GDB when it is not in the tree.
func (a *DebuggerApp) pickGdbFallbackLeaf() *termui.Node {
	if a.tab == nil {
		return nil
	}
	// Prefer the remembered gdb leaf even if it currently shows something else.
	if leaf := a.tab.LeafMark(leafMarkGDB); leaf != nil {
		return leaf
	}
	tree := a.tab.ActiveTree()
	if tree == nil {
		return nil
	}
	codeLeaf := a.tab.LeafMark(leafMarkCode)
	for _, n := range termui.CollectLeaves(tree.Root()) {
		if n != codeLeaf {
			return n
		}
	}
	return a.tab.TopLeftLeaf()
}

// activateGdbPane focuses the pane that holds GDB (restoring it on the remembered
// leaf if needed). Used when entering insert mode with 'i'.
func (a *DebuggerApp) activateGdbPane() {
	if a.tab == nil || a.gdbWidget == nil {
		return
	}
	leaf := a.findGdbLeaf()
	if leaf == nil {
		leaf = a.pickGdbFallbackLeaf()
	}
	if leaf == nil {
		return
	}
	if leaf.GetWidget() != a.gdbWidget {
		leaf.SetWidget(a.gdbWidget)
	}
	_ = a.tab.FocusLeaf(leaf)
	a.tab.SetLeafMark(leafMarkGDB, leaf)
}

// activateGdbInsertMode focuses the GDB pane then enters insert mode ('i').
func (a *DebuggerApp) activateGdbInsertMode() {
	a.activateGdbPane()
	a.EnterInsertMode()
}

// onEscape leaves insert/normal Esc handling. When AppState.EscToCode is set
// (default), focuses the CodeWidget leaf; otherwise only leaves insert → normal.
func (a *DebuggerApp) onEscape() {
	if a.State().EscToCode() {
		a.activateCodePane()
		return
	}
	if a.tab != nil {
		a.tab.SetInsertActive(false)
	}
	a.SetMode(platform.ModeNormal)
	a.RequestRedraw()
}

// activateCodePane leaves insert mode, focuses the pane that holds a CodeWidget,
// or places the active CodeWidget on the top-left leaf if none exists.
// Does not steal the GDB pane when a code pane already exists elsewhere.
func (a *DebuggerApp) activateCodePane() {
	if a.tab == nil {
		return
	}
	a.tab.SetInsertActive(false)
	a.SetMode(platform.ModeNormal)

	cw := a.activeCodeWidget()
	if cw == nil {
		a.RequestRedraw()
		return
	}

	leaf := a.findCodeLeaf()
	if leaf == nil {
		leaf = a.tab.TopLeftLeaf()
	}
	if leaf == nil {
		a.RequestRedraw()
		return
	}

	if leaf.GetWidget() != cw {
		leaf.SetWidget(cw)
	}
	_ = a.tab.FocusLeaf(leaf)
	a.tab.SetLeafMark(leafMarkCode, leaf)
	a.RequestRedraw()
}

// sendGdbExec sends a CLI exec command (next/step) on the shared GDB PTY.
func (a *DebuggerApp) sendGdbExec(cmd string) {
	if a.gdbWidget == nil || cmd == "" {
		return
	}
	sess := a.gdbWidget.Session()
	if sess == nil {
		return
	}
	a.State().WithPTYOwner(platform.PTYOwnerUI, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = sess.WithWrite(ctx, func(pw core.PTYWriter) error {
			return pw.Send(cmd)
		})
	})
}
