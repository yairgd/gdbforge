package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/termui"
)

func (w *LayoutShell) placeCodeInSlot(cw *widgets.CodeWidget) {
	lay := w.Layout()
	h := w.host
	if cw == nil || lay == nil || h == nil {
		return
	}
	h.BufsSetPrimary(cw)
	if h.AsmPreferAsm() && !h.AsmHasSplit() {
		return
	}
	if h.AsmHasSplit() {
		asm := lay.LeafMark(leafMarkAsm)
		if asm != nil && w.focusedLeaf() == asm {
			if leaf := w.findCodeLeaf(); leaf != nil && !w.isGdbLeaf(leaf) && leaf != asm {
				if isAssemblyWidget(leaf.GetWidget()) && h.AsmPreferAsm() {
					return
				}
				leaf.SetWidget(cw)
				lay.SetLeafMark(leafMarkCode, leaf)
			}
			return
		}
		if isAssemblyWidget(h.FocusedWidget()) {
			if leaf := w.findCodeLeaf(); leaf != nil && !w.isGdbLeaf(leaf) && !isAssemblyWidget(leaf.GetWidget()) {
				leaf.SetWidget(cw)
				lay.SetLeafMark(leafMarkCode, leaf)
			}
			return
		}
	}
	if !w.isGdbLeaf(w.focusedLeaf()) {
		if isAssemblyWidget(h.FocusedWidget()) {
			if h.FocusedWidget() != cw {
				_ = lay.ReplaceFocusedWidget(cw)
			}
			w.rememberCodeLeafFromFocus()
			return
		}
		if focused := h.focusedCode(); focused != nil {
			if focused != cw {
				_ = lay.ReplaceFocusedWidget(cw)
			}
			w.rememberCodeLeafFromFocus()
			return
		}
		if _, ok := h.FocusedWidget().(*widgets.LogoWidget); ok {
			_ = lay.ReplaceFocusedWidget(cw)
			w.rememberCodeLeafFromFocus()
			return
		}
	}
	if leaf := w.findCodeLeaf(); leaf != nil && !w.isGdbLeaf(leaf) {
		if h.AsmHasSplit() && lay.LeafMark(leafMarkAsm) == leaf {
			return
		}
		if isAssemblyWidget(leaf.GetWidget()) && h.AsmPreferAsm() {
			return
		}
		leaf.SetWidget(cw)
		lay.SetLeafMark(leafMarkCode, leaf)
		if lay.LeafMark(leafMarkAsm) == leaf {
			lay.SetLeafMark(leafMarkAsm, nil)
		}
		return
	}
	if lay.ReplaceMatchingLeafWidget(cw, isSourceCodeSlot) {
		lay.SetLeafMark(leafMarkCode, lay.FindLeaf(isSourceCodeSlot))
	}
}

func (w *LayoutShell) placeLogoInCodeSlot() {
	lay := w.Layout()
	h := w.host
	if lay == nil || h == nil {
		return
	}
	logo := h.LogoWidget()
	if logo == nil {
		logo = widgets.NewLogoWidget()
		h.SetLogoWidget(logo)
	}
	if _, ok := h.FocusedWidget().(*widgets.CodeWidget); ok && !w.isGdbLeaf(w.focusedLeaf()) {
		_ = lay.ReplaceFocusedWidget(logo)
		lay.SetLeafMark(leafMarkCode, lay.FindLeaf(isCodeSlot))
		return
	}
	if lay.ReplaceMatchingLeafWidget(logo, isCodeSlot) {
		lay.SetLeafMark(leafMarkCode, lay.FindLeaf(isCodeSlot))
	}
}

func (w *LayoutShell) swapFocusedWidget(wid termui.Widget) bool {
	lay := w.Layout()
	h := w.host
	if lay == nil || wid == nil || h == nil {
		return false
	}
	if w.isGdbLeaf(w.focusedLeaf()) && wid != h.GDBWidget() {
		return false
	}
	prev := h.FocusedWidget()
	if prev == wid {
		return false
	}
	if !lay.ReplaceFocusedWidget(wid) {
		return false
	}
	if prev != nil {
		w.pushWidgetJump(prev)
	}
	w.rememberCodeLeafFromFocus()
	return true
}

func (w *LayoutShell) pushWidgetJump(wid termui.Widget) {
	if wid == nil {
		return
	}
	if n := len(w.widgetJump); n > 0 && w.widgetJump[n-1] == wid {
		return
	}
	w.widgetJump = append(w.widgetJump, wid)
	if len(w.widgetJump) > widgetJumpMax {
		w.widgetJump = w.widgetJump[len(w.widgetJump)-widgetJumpMax:]
	}
}

func (w *LayoutShell) JumpBack(args ...any) {
	lay := w.Layout()
	h := w.host
	if lay == nil || h == nil || len(w.widgetJump) == 0 {
		return
	}
	prev := w.widgetJump[len(w.widgetJump)-1]
	if w.isGdbLeaf(w.focusedLeaf()) && prev != h.GDBWidget() {
		return
	}
	w.widgetJump = w.widgetJump[:len(w.widgetJump)-1]
	if lay.ReplaceFocusedWidget(prev) {
		h.RequestFrame()
	}
}
