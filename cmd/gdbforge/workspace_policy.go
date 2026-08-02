package main

import (
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// findCodeLeaf returns the remembered code leaf if still valid, else any leaf
// currently showing a CodeWidget or LogoWidget (not a dedicated asm split pane).
func (w *Workspace) findCodeLeaf() *termui.Node {
	tab := w.Tab()
	if tab == nil {
		return nil
	}
	if leaf := tab.LeafMark(leafMarkCode); leaf != nil {
		wid := leaf.GetWidget()
		if isSourceCodeSlot(wid) {
			return leaf
		}
		// Shared location leaf showing Assembly (:b asm / autoAsm).
		// Never clear the code mark just because the leaf currently holds Asm —
		// that used to make hasSplit() look true and block reclaiming Code.
		if isAssemblyWidget(wid) {
			if tab.LeafMark(leafMarkAsm) == leaf {
				tab.SetLeafMark(leafMarkAsm, nil)
			}
			return leaf
		}
	}
	if leaf := tab.FindLeaf(isSourceCodeSlot); leaf != nil {
		tab.SetLeafMark(leafMarkCode, leaf)
		return leaf
	}
	// Heal contamination: asm mark on the only location leaf (no Code/Logo).
	if asm := tab.LeafMark(leafMarkAsm); asm != nil && isAssemblyWidget(asm.GetWidget()) {
		tab.SetLeafMark(leafMarkCode, asm)
		tab.SetLeafMark(leafMarkAsm, nil)
		return asm
	}
	tab.SetLeafMark(leafMarkCode, nil)
	return nil
}

// rememberCodeLeafFromFocus updates code/gdb marks and the Esc "last" mark.
// Non-code/non-gdb focus becomes the Esc restore target; Code clears that
// target; GDB does not overwrite it (so Esc after `i` can return to e.g. BPs).
func (w *Workspace) rememberCodeLeafFromFocus() {
	tab := w.Tab()
	if tab == nil || w.app == nil {
		return
	}
	tree := tab.ActiveTree()
	if tree == nil {
		return
	}
	leaf := tree.FocusedLeaf()
	if leaf == nil {
		return
	}
	wid := leaf.GetWidget()
	switch {
	case isAssemblyWidget(wid):
		codeLeaf := tab.LeafMark(leafMarkCode)
		// Dedicated :vs/:sp asm only when distinct from the location leaf.
		if codeLeaf != nil && codeLeaf != leaf {
			tab.SetLeafMark(leafMarkAsm, leaf)
			break
		}
		// Shared location leaf (:b asm / autoAsm) — keep the code mark.
		tab.SetLeafMark(leafMarkCode, leaf)
		tab.SetLeafMark(leafMarkAsm, nil)
		tab.SetLeafMark(leafMarkLast, nil)
	case isSourceCodeSlot(wid):
		tab.SetLeafMark(leafMarkCode, leaf)
		tab.SetLeafMark(leafMarkLast, nil)
	case isCodeSlot(wid):
		// Single-pane asm occupying the code leaf.
		tab.SetLeafMark(leafMarkCode, leaf)
		tab.SetLeafMark(leafMarkAsm, nil)
		tab.SetLeafMark(leafMarkLast, nil)
	case wid == w.app.gdbWidget:
		tab.SetLeafMark(leafMarkGDB, leaf)
	default:
		tab.SetLeafMark(leafMarkLast, leaf)
	}
}

// focusedLeaf returns the focused leaf in the active tab tree.
func (w *Workspace) focusedLeaf() *termui.Node {
	tab := w.Tab()
	if tab == nil {
		return nil
	}
	tree := tab.ActiveTree()
	if tree == nil {
		return nil
	}
	return tree.FocusedLeaf()
}

// isGdbLeaf reports whether leaf is the layout's GDB slot (marked "gdb" or
// currently showing gdbWidget). That leaf must not host other widgets.
func (w *Workspace) isGdbLeaf(leaf *termui.Node) bool {
	tab := w.Tab()
	if leaf == nil || tab == nil || w.app == nil {
		return false
	}
	if m := tab.LeafMark(leafMarkGDB); m != nil && m == leaf {
		return true
	}
	return w.app.gdbWidget != nil && leaf.GetWidget() == w.app.gdbWidget
}

// focusIsCodeOrGdb reports whether the focused pane is Code/Logo or GDB (or empty).
// Other panes keep their own Up/Down/Space handling.
func (w *Workspace) focusIsCodeOrGdb() bool {
	if w.app == nil {
		return true
	}
	wid := w.app.focusedWidget()
	if wid == nil {
		return true
	}
	if isCodeSlot(wid) {
		return true
	}
	return wid == w.app.gdbWidget
}

// activateLastOrCodePane focuses the remembered non-code/non-gdb leaf when
// still valid; otherwise falls back to the Code pane (EscToCode path).
func (w *Workspace) activateLastOrCodePane() {
	tab := w.Tab()
	if tab == nil || w.app == nil {
		return
	}
	if leaf := tab.LeafMark(leafMarkLast); leaf != nil {
		wid := leaf.GetWidget()
		if wid != nil && !isCodeSlot(wid) && wid != w.app.gdbWidget {
			tab.SetInsertActive(false)
			w.app.SetMode(platform.ModeNormal)
			_ = tab.FocusLeaf(leaf)
			w.app.RequestRedraw()
			return
		}
	}
	w.FocusCode()
}

// findGdbLeaf returns the remembered GDB leaf if it still shows GDB, else any
// leaf currently showing the GDBWidget.
func (w *Workspace) findGdbLeaf() *termui.Node {
	tab := w.Tab()
	if tab == nil || w.app == nil || w.app.gdbWidget == nil {
		return nil
	}
	if leaf := tab.LeafMark(leafMarkGDB); leaf != nil && leaf.GetWidget() == w.app.gdbWidget {
		return leaf
	}
	leaf := tab.FindLeaf(func(wid termui.Widget) bool { return wid == w.app.gdbWidget })
	tab.SetLeafMark(leafMarkGDB, leaf)
	return leaf
}

// pickGdbFallbackLeaf chooses a leaf to host GDB when it is not in the tree.
func (w *Workspace) pickGdbFallbackLeaf() *termui.Node {
	tab := w.Tab()
	if tab == nil {
		return nil
	}
	// Prefer the remembered gdb leaf even if it currently shows something else.
	if leaf := tab.LeafMark(leafMarkGDB); leaf != nil {
		return leaf
	}
	tree := tab.ActiveTree()
	if tree == nil {
		return nil
	}
	codeLeaf := tab.LeafMark(leafMarkCode)
	for _, n := range termui.CollectLeaves(tree.Root()) {
		if n != codeLeaf {
			return n
		}
	}
	return tab.TopLeftLeaf()
}

// activateGdbPane focuses the pane that holds GDB (restoring it on the remembered
// leaf if needed). Used when entering insert mode with 'i'.
func (w *Workspace) activateGdbPane() {
	tab := w.Tab()
	if tab == nil || w.app == nil || w.app.gdbWidget == nil {
		return
	}
	leaf := w.findGdbLeaf()
	if leaf == nil {
		leaf = w.pickGdbFallbackLeaf()
	}
	if leaf == nil {
		return
	}
	if leaf.GetWidget() != w.app.gdbWidget {
		leaf.SetWidget(w.app.gdbWidget)
	}
	_ = tab.FocusLeaf(leaf)
	tab.SetLeafMark(leafMarkGDB, leaf)
}

// activateGdbInsertMode focuses the GDB pane then enters insert mode ('i').
func (w *Workspace) activateGdbInsertMode() {
	if w.app == nil {
		return
	}
	w.rememberCodeLeafFromFocus()
	w.activateGdbPane()
	w.app.EnterInsertMode()
}

// FocusCode leaves insert mode and focuses the code slot (Logo/Code/Asm).
func (w *Workspace) FocusCode() {
	tab := w.Tab()
	if tab == nil || w.app == nil {
		return
	}
	tab.SetInsertActive(false)
	w.app.SetMode(platform.ModeNormal)

	leaf := w.findCodeLeaf()
	if leaf == nil {
		leaf = tab.TopLeftLeaf()
	}
	if leaf == nil {
		w.app.RequestRedraw()
		return
	}

	a := w.app
	if aw := a.asm.Widget(); a.asm.PreferAsm() && aw != nil && !a.asm.hasSplit() {
		if leaf.GetWidget() != aw {
			leaf.SetWidget(aw)
		}
	} else if cw := a.activeCodeWidget(); cw != nil && leaf.GetWidget() != cw {
		leaf.SetWidget(cw)
	}
	_ = tab.FocusLeaf(leaf)
	tab.SetLeafMark(leafMarkCode, leaf)
	a.RequestRedraw()
}
