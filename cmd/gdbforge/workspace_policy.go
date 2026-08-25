package main

import (
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

func (w *LayoutShell) findCodeLeaf() *termui.Node {
	tab := w.Tab()
	if tab == nil {
		return nil
	}
	if leaf := tab.LeafMark(leafMarkCode); leaf != nil {
		wid := leaf.GetWidget()
		if isSourceCodeSlot(wid) {
			return leaf
		}
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
	if asm := tab.LeafMark(leafMarkAsm); asm != nil && isAssemblyWidget(asm.GetWidget()) {
		tab.SetLeafMark(leafMarkCode, asm)
		tab.SetLeafMark(leafMarkAsm, nil)
		return asm
	}
	tab.SetLeafMark(leafMarkCode, nil)
	return nil
}

func (w *LayoutShell) rememberCodeLeafFromFocus() {
	tab := w.Tab()
	h := w.host
	if tab == nil || h == nil {
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
	gdb := h.GDBWidget()
	switch {
	case isAssemblyWidget(wid):
		codeLeaf := tab.LeafMark(leafMarkCode)
		if codeLeaf != nil && codeLeaf != leaf {
			tab.SetLeafMark(leafMarkAsm, leaf)
			break
		}
		tab.SetLeafMark(leafMarkCode, leaf)
		tab.SetLeafMark(leafMarkAsm, nil)
		tab.SetLeafMark(leafMarkLast, nil)
	case isSourceCodeSlot(wid):
		tab.SetLeafMark(leafMarkCode, leaf)
		tab.SetLeafMark(leafMarkLast, nil)
	case isCodeSlot(wid):
		tab.SetLeafMark(leafMarkCode, leaf)
		tab.SetLeafMark(leafMarkAsm, nil)
		tab.SetLeafMark(leafMarkLast, nil)
	case wid == gdb:
		tab.SetLeafMark(leafMarkGDB, leaf)
	default:
		tab.SetLeafMark(leafMarkLast, leaf)
	}
}

func (w *LayoutShell) focusedLeaf() *termui.Node {
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

func (w *LayoutShell) isGdbLeaf(leaf *termui.Node) bool {
	tab := w.Tab()
	h := w.host
	if leaf == nil || tab == nil || h == nil {
		return false
	}
	if m := tab.LeafMark(leafMarkGDB); m != nil && m == leaf {
		return true
	}
	gdb := h.GDBWidget()
	return gdb != nil && leaf.GetWidget() == gdb
}

func (w *LayoutShell) focusIsCodeOrGdb() bool {
	h := w.host
	if h == nil {
		return true
	}
	wid := h.FocusedWidget()
	if wid == nil {
		return true
	}
	if isCodeSlot(wid) {
		return true
	}
	return wid == h.GDBWidget()
}

func (w *LayoutShell) activateLastOrCodePane() {
	tab := w.Tab()
	h := w.host
	if tab == nil || h == nil {
		return
	}
	gdb := h.GDBWidget()
	if leaf := tab.LeafMark(leafMarkLast); leaf != nil {
		wid := leaf.GetWidget()
		if wid != nil && !isCodeSlot(wid) && wid != gdb {
			tab.SetInsertActive(false)
			h.SetMode(platform.ModeNormal)
			_ = tab.FocusLeaf(leaf)
			h.RequestRedraw()
			return
		}
	}
	w.FocusCode()
}

func (w *LayoutShell) findGdbLeaf() *termui.Node {
	tab := w.Tab()
	h := w.host
	gdb := h.GDBWidget()
	if tab == nil || h == nil || gdb == nil {
		return nil
	}
	if leaf := tab.LeafMark(leafMarkGDB); leaf != nil && leaf.GetWidget() == gdb {
		return leaf
	}
	leaf := tab.FindLeaf(func(wid termui.Widget) bool { return wid == gdb })
	tab.SetLeafMark(leafMarkGDB, leaf)
	return leaf
}

func (w *LayoutShell) pickGdbFallbackLeaf() *termui.Node {
	tab := w.Tab()
	if tab == nil {
		return nil
	}
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

func (w *LayoutShell) activateGdbPane() {
	tab := w.Tab()
	h := w.host
	gdb := h.GDBWidget()
	if tab == nil || h == nil || gdb == nil {
		return
	}
	leaf := w.findGdbLeaf()
	if leaf == nil {
		leaf = w.pickGdbFallbackLeaf()
	}
	if leaf == nil {
		return
	}
	if leaf.GetWidget() != gdb {
		leaf.SetWidget(gdb)
	}
	_ = tab.FocusLeaf(leaf)
	tab.SetLeafMark(leafMarkGDB, leaf)
}

func (w *LayoutShell) activateGdbInsertMode() {
	if w.host == nil {
		return
	}
	w.rememberCodeLeafFromFocus()
	w.activateGdbPane()
	w.host.EnterInsertMode()
}

func (w *LayoutShell) FocusCode() {
	tab := w.Tab()
	h := w.host
	if tab == nil || h == nil {
		return
	}
	tab.SetInsertActive(false)
	h.SetMode(platform.ModeNormal)

	leaf := w.findCodeLeaf()
	if leaf == nil {
		leaf = tab.TopLeftLeaf()
	}
	if leaf == nil {
		h.RequestRedraw()
		return
	}

	if aw := h.AsmWidget(); h.AsmPreferAsm() && aw != nil && !h.AsmHasSplit() {
		if leaf.GetWidget() != aw {
			leaf.SetWidget(aw)
		}
	} else if cw := h.ActiveCodeWidget(); cw != nil && leaf.GetWidget() != cw {
		leaf.SetWidget(cw)
	}
	_ = tab.FocusLeaf(leaf)
	tab.SetLeafMark(leafMarkCode, leaf)
	h.RequestRedraw()
}
