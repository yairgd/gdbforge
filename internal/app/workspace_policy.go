package app

import (
	"github.com/yairgd/termforge"
	"github.com/yairgd/termforge/platform"
)

func (w *LayoutShell) findCodeLeaf() *termforge.Node {
	lay := w.Layout()
	if lay == nil {
		return nil
	}
	if leaf := lay.LeafMark(leafMarkCode); leaf != nil {
		wid := leaf.GetWidget()
		if isSourceCodeSlot(wid) {
			return leaf
		}
		if isAssemblyWidget(wid) {
			if lay.LeafMark(leafMarkAsm) == leaf {
				lay.SetLeafMark(leafMarkAsm, nil)
			}
			return leaf
		}
	}
	if leaf := lay.FindLeaf(isSourceCodeSlot); leaf != nil {
		lay.SetLeafMark(leafMarkCode, leaf)
		return leaf
	}
	if asm := lay.LeafMark(leafMarkAsm); asm != nil && isAssemblyWidget(asm.GetWidget()) {
		lay.SetLeafMark(leafMarkCode, asm)
		lay.SetLeafMark(leafMarkAsm, nil)
		return asm
	}
	lay.SetLeafMark(leafMarkCode, nil)
	return nil
}

func (w *LayoutShell) rememberCodeLeafFromFocus() {
	lay := w.Layout()
	h := w.host
	if lay == nil || h == nil {
		return
	}
	leaf := lay.FocusedLeaf()
	if leaf == nil {
		return
	}
	wid := leaf.GetWidget()
	gdb := h.GDBWidget()
	switch {
	case isAssemblyWidget(wid):
		codeLeaf := lay.LeafMark(leafMarkCode)
		if codeLeaf != nil && codeLeaf != leaf {
			lay.SetLeafMark(leafMarkAsm, leaf)
			break
		}
		lay.SetLeafMark(leafMarkCode, leaf)
		lay.SetLeafMark(leafMarkAsm, nil)
		lay.SetLeafMark(leafMarkLast, nil)
	case isSourceCodeSlot(wid):
		lay.SetLeafMark(leafMarkCode, leaf)
		lay.SetLeafMark(leafMarkLast, nil)
	case isCodeSlot(wid):
		lay.SetLeafMark(leafMarkCode, leaf)
		lay.SetLeafMark(leafMarkAsm, nil)
		lay.SetLeafMark(leafMarkLast, nil)
	case wid == gdb:
		lay.SetLeafMark(leafMarkGDB, leaf)
	default:
		lay.SetLeafMark(leafMarkLast, leaf)
	}
}

func (w *LayoutShell) focusedLeaf() *termforge.Node {
	lay := w.Layout()
	if lay == nil {
		return nil
	}
	return lay.FocusedLeaf()
}

func (w *LayoutShell) isGdbLeaf(leaf *termforge.Node) bool {
	lay := w.Layout()
	h := w.host
	if leaf == nil || lay == nil || h == nil {
		return false
	}
	if m := lay.LeafMark(leafMarkGDB); m != nil && m == leaf {
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
	lay := w.Layout()
	h := w.host
	if lay == nil || h == nil {
		return
	}
	gdb := h.GDBWidget()
	if leaf := lay.LeafMark(leafMarkLast); leaf != nil {
		wid := leaf.GetWidget()
		if wid != nil && !isCodeSlot(wid) && wid != gdb {
			lay.SetInsertActive(false)
			h.SetMode(platform.ModeNormal)
			_ = lay.FocusLeaf(leaf)
			h.RequestRedraw()
			return
		}
	}
	w.FocusCode()
}

func (w *LayoutShell) findGdbLeaf() *termforge.Node {
	lay := w.Layout()
	h := w.host
	gdb := h.GDBWidget()
	if lay == nil || h == nil || gdb == nil {
		return nil
	}
	if leaf := lay.LeafMark(leafMarkGDB); leaf != nil && leaf.GetWidget() == gdb {
		return leaf
	}
	leaf := lay.FindLeaf(func(wid termforge.Widget) bool { return wid == gdb })
	lay.SetLeafMark(leafMarkGDB, leaf)
	return leaf
}

func (w *LayoutShell) pickGdbFallbackLeaf() *termforge.Node {
	lay := w.Layout()
	if lay == nil {
		return nil
	}
	if leaf := lay.LeafMark(leafMarkGDB); leaf != nil {
		return leaf
	}
	codeLeaf := lay.LeafMark(leafMarkCode)
	for _, n := range termforge.CollectLeaves(lay.Root()) {
		if n != codeLeaf {
			return n
		}
	}
	return lay.TopLeftLeaf()
}

func (w *LayoutShell) activateGdbPane() {
	lay := w.Layout()
	h := w.host
	gdb := h.GDBWidget()
	if lay == nil || h == nil || gdb == nil {
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
	_ = lay.FocusLeaf(leaf)
	lay.SetLeafMark(leafMarkGDB, leaf)
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
	lay := w.Layout()
	h := w.host
	if lay == nil || h == nil {
		return
	}
	lay.SetInsertActive(false)
	h.SetMode(platform.ModeNormal)

	leaf := w.findCodeLeaf()
	if leaf == nil {
		leaf = lay.TopLeftLeaf()
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
	_ = lay.FocusLeaf(leaf)
	lay.SetLeafMark(leafMarkCode, leaf)
	h.RequestRedraw()
}
