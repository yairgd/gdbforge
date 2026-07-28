package main

import (
	"context"
	"strings"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// Named leaf marks on the active WidgetTree (role names live only in cmd/gdbforge).
const (
	leafMarkCode = "code"
	leafMarkGDB  = "gdb"
	leafMarkAsm  = "asm"
	// leafMarkLast is the Esc restore target when the user last focused a pane
	// that is neither Code nor GDB (breakpoints, callstack, …). Focusing Code
	// clears it; focusing GDB leaves it unchanged.
	leafMarkLast = "last"
)

// activeCodeWidget returns the CodeWidget buffer Esc / global keys should drive.
func (a *DebuggerApp) activeCodeWidget() *widgets.CodeWidget {
	if cw := a.focusedCode(); cw != nil {
		return cw
	}
	if path := a.Debug().CurrentFile(); path != "" {
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

// isCodeSlot is the startup code leaf: Logo / Code, or single-pane Assembly
// when there is no dedicated :vs asm / :sp asm leaf.
func isCodeSlot(w termui.Widget) bool {
	if isCodeWidget(w) {
		return true
	}
	if _, ok := w.(*widgets.LogoWidget); ok {
		return true
	}
	return isAssemblyWidget(w)
}

// isSourceCodeSlot is a leaf that shows source (or the logo placeholder).
func isSourceCodeSlot(w termui.Widget) bool {
	if isCodeWidget(w) {
		return true
	}
	_, ok := w.(*widgets.LogoWidget)
	return ok
}

// findCodeLeaf returns the remembered code leaf if still valid, else any leaf
// currently showing a CodeWidget or LogoWidget (not a dedicated asm split pane).
func (a *DebuggerApp) findCodeLeaf() *termui.Node {
	if a.tab == nil {
		return nil
	}
	if leaf := a.tab.LeafMark(leafMarkCode); leaf != nil {
		w := leaf.GetWidget()
		if isSourceCodeSlot(w) {
			return leaf
		}
		// Single-pane :b asm still owns the code mark.
		if isAssemblyWidget(w) && !a.hasAsmSplit() {
			return leaf
		}
	}
	leaf := a.tab.FindLeaf(isSourceCodeSlot)
	a.tab.SetLeafMark(leafMarkCode, leaf)
	return leaf
}

// hasAsmSplit reports a dedicated Assembly leaf from :vs asm / :sp asm.
func (a *DebuggerApp) hasAsmSplit() bool {
	if a == nil || a.tab == nil || a.assemblyWidget == nil {
		return false
	}
	leaf := a.tab.LeafMark(leafMarkAsm)
	return leaf != nil && leaf.GetWidget() == a.assemblyWidget
}

// findAsmLeaf returns the dedicated asm split leaf, if any.
func (a *DebuggerApp) findAsmLeaf() *termui.Node {
	if a.tab == nil {
		return nil
	}
	if leaf := a.tab.LeafMark(leafMarkAsm); leaf != nil && isAssemblyWidget(leaf.GetWidget()) {
		return leaf
	}
	return a.tab.FindLeaf(isAssemblyWidget)
}

// rememberCodeLeafFromFocus updates code/gdb marks and the Esc "last" mark.
// Non-code/non-gdb focus becomes the Esc restore target; Code clears that
// target; GDB does not overwrite it (so Esc after `i` can return to e.g. BPs).
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
	w := leaf.GetWidget()
	switch {
	case isAssemblyWidget(w):
		a.tab.SetLeafMark(leafMarkAsm, leaf)
	case isSourceCodeSlot(w):
		a.tab.SetLeafMark(leafMarkCode, leaf)
		a.tab.SetLeafMark(leafMarkLast, nil)
	case isCodeSlot(w):
		// Single-pane asm occupying the code leaf.
		a.tab.SetLeafMark(leafMarkCode, leaf)
		a.tab.SetLeafMark(leafMarkLast, nil)
	case w == a.gdbWidget:
		a.tab.SetLeafMark(leafMarkGDB, leaf)
	default:
		a.tab.SetLeafMark(leafMarkLast, leaf)
	}
}

// focusedLeaf returns the focused leaf in the active tab tree.
func (a *DebuggerApp) focusedLeaf() *termui.Node {
	if a.tab == nil {
		return nil
	}
	tree := a.tab.ActiveTree()
	if tree == nil {
		return nil
	}
	return tree.FocusedLeaf()
}

// isGdbLeaf reports whether leaf is the layout's GDB slot (marked "gdb" or
// currently showing gdbWidget). That leaf must not host other widgets.
func (a *DebuggerApp) isGdbLeaf(leaf *termui.Node) bool {
	if leaf == nil || a.tab == nil {
		return false
	}
	if m := a.tab.LeafMark(leafMarkGDB); m != nil && m == leaf {
		return true
	}
	return a.gdbWidget != nil && leaf.GetWidget() == a.gdbWidget
}

// focusIsCodeOrGdb reports whether the focused pane is Code/Logo or GDB (or empty).
// Other panes keep their own Up/Down/Space handling.
func (a *DebuggerApp) focusIsCodeOrGdb() bool {
	w := a.focusedWidget()
	if w == nil {
		return true
	}
	if isCodeSlot(w) {
		return true
	}
	return w == a.gdbWidget
}

// activateLastOrCodePane focuses the remembered non-code/non-gdb leaf when
// still valid; otherwise falls back to the Code pane (EscToCode path).
func (a *DebuggerApp) activateLastOrCodePane() {
	if a.tab == nil {
		return
	}
	if leaf := a.tab.LeafMark(leafMarkLast); leaf != nil {
		w := leaf.GetWidget()
		if w != nil && !isCodeSlot(w) && w != a.gdbWidget {
			a.tab.SetInsertActive(false)
			a.SetMode(platform.ModeNormal)
			_ = a.tab.FocusLeaf(leaf)
			a.RequestRedraw()
			return
		}
	}
	a.activateCodePane()
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
	a.rememberCodeLeafFromFocus()
	a.activateGdbPane()
	a.EnterInsertMode()
}

// onEscape leaves insert/normal Esc handling. When AppState.EscToCode is set
// (default), focuses the last non-code/non-gdb pane if one was active, else
// the CodeWidget leaf; otherwise only leaves insert → normal.
func (a *DebuggerApp) onEscape() {
	if a.State().EscToCode() {
		a.activateLastOrCodePane()
		return
	}
	if a.tab != nil {
		a.tab.SetInsertActive(false)
	}
	a.SetMode(platform.ModeNormal)
	a.RequestRedraw()
}

// activateCodePane leaves insert mode and focuses the code slot (Logo/Code/Asm).
func (a *DebuggerApp) activateCodePane() {
	if a.tab == nil {
		return
	}
	a.tab.SetInsertActive(false)
	a.SetMode(platform.ModeNormal)

	leaf := a.findCodeLeaf()
	if leaf == nil {
		leaf = a.tab.TopLeftLeaf()
	}
	if leaf == nil {
		a.RequestRedraw()
		return
	}

	if a.preferAsm && a.assemblyWidget != nil && !a.hasAsmSplit() {
		if leaf.GetWidget() != a.assemblyWidget {
			leaf.SetWidget(a.assemblyWidget)
		}
	} else if cw := a.activeCodeWidget(); cw != nil && leaf.GetWidget() != cw {
		leaf.SetWidget(cw)
	}
	_ = a.tab.FocusLeaf(leaf)
	a.tab.SetLeafMark(leafMarkCode, leaf)
	a.RequestRedraw()
}

// sendGdbExec sends an execution command on the shared debugger PTY.
// GDB: prefer MI -exec-* so stops update Code via *stopped without dumping a
// CLI source listing into the console. Delve: keep CLI (no MI mapping).
func (a *DebuggerApp) sendGdbExec(cmd string) {
	if a.gdbWidget == nil || cmd == "" {
		return
	}
	sess := a.GDB()
	if sess == nil {
		return
	}
	sendCmd := strings.TrimSpace(cmd)
	if a.isDLV() {
		switch sendCmd {
		case "finish":
			sendCmd = "stepout"
		case "run", "start":
			sendCmd = "restart"
		}
		if isDlvRunCmd(sendCmd) && a.State() != nil {
			a.Debug().SetInferiorRunning(true)
		}
	} else {
		sendCmd = gdb.CLIExecToMI(sendCmd)
		// Arm before ^running arrives so Space-break while the inferior is
		// mid-continue still interrupts and installs the BP.
		if isDlvRunCmd(strings.TrimSpace(cmd)) && a.State() != nil {
			a.Debug().SetInferiorRunning(true)
		}
	}
	a.State().WithPTYOwner(platform.PTYOwnerUI, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = sess.WithWrite(ctx, func(pw core.PTYWriter) error {
			return pw.Send(sendCmd)
		})
	})
}
