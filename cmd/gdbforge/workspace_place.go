package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/termui"
)

// placeCodeInSlot puts w into the location leaf, replacing Logo or Code.
// Never places onto the fixed GDB leaf or a dedicated asm split leaf.
// Sticky :b asm blocks reclaim; autoAsm does not (source can reclaim the leaf).
func (w *Workspace) placeCodeInSlot(cw *widgets.CodeWidget) {
	tab := w.Tab()
	if cw == nil || tab == nil || w.app == nil {
		return
	}
	a := w.app
	a.bufs.setPrimary(cw)
	// :b asm owns the location leaf until :b code — do not swap Assembly out.
	if a.asm.PreferAsm() && !a.asm.hasSplit() {
		return
	}
	if a.asm.hasSplit() {
		asm := tab.LeafMark(leafMarkAsm)
		if asm != nil && w.focusedLeaf() == asm {
			// Focus is on dedicated asm — update the code leaf in place.
			if leaf := w.findCodeLeaf(); leaf != nil && !w.isGdbLeaf(leaf) && leaf != asm {
				if isAssemblyWidget(leaf.GetWidget()) && a.asm.PreferAsm() {
					return
				}
				leaf.SetWidget(cw)
				tab.SetLeafMark(leafMarkCode, leaf)
			}
			return
		}
		if isAssemblyWidget(a.focusedWidget()) {
			if leaf := w.findCodeLeaf(); leaf != nil && !w.isGdbLeaf(leaf) && !isAssemblyWidget(leaf.GetWidget()) {
				leaf.SetWidget(cw)
				tab.SetLeafMark(leafMarkCode, leaf)
			}
			return
		}
	}
	if !w.isGdbLeaf(w.focusedLeaf()) {
		if isAssemblyWidget(a.focusedWidget()) {
			// Shared leaf showing Asm (autoAsm) — reclaim with Code.
			if a.focusedWidget() != cw {
				_ = tab.ReplaceFocusedWidget(cw)
			}
			w.rememberCodeLeafFromFocus()
			return
		}
		if focused := a.focusedCode(); focused != nil {
			if focused != cw {
				_ = tab.ReplaceFocusedWidget(cw)
			}
			w.rememberCodeLeafFromFocus()
			return
		}
		if _, ok := a.focusedWidget().(*widgets.LogoWidget); ok {
			_ = tab.ReplaceFocusedWidget(cw)
			w.rememberCodeLeafFromFocus()
			return
		}
	}
	if leaf := w.findCodeLeaf(); leaf != nil && !w.isGdbLeaf(leaf) {
		if a.asm.hasSplit() && tab.LeafMark(leafMarkAsm) == leaf {
			return
		}
		if isAssemblyWidget(leaf.GetWidget()) && a.asm.PreferAsm() {
			return
		}
		leaf.SetWidget(cw)
		tab.SetLeafMark(leafMarkCode, leaf)
		if tab.LeafMark(leafMarkAsm) == leaf {
			tab.SetLeafMark(leafMarkAsm, nil)
		}
		return
	}
	if tab.ReplaceMatchingLeafWidget(cw, isSourceCodeSlot) {
		tab.SetLeafMark(leafMarkCode, tab.FindLeaf(isSourceCodeSlot))
	}
}

// placeLogoInCodeSlot puts the startup logo back in the code leaf (after kill).
func (w *Workspace) placeLogoInCodeSlot() {
	tab := w.Tab()
	if tab == nil || w.app == nil {
		return
	}
	a := w.app
	logo := a.logoWidget
	if logo == nil {
		logo = widgets.NewLogoWidget()
		a.logoWidget = logo
	}
	if _, ok := a.focusedWidget().(*widgets.CodeWidget); ok && !w.isGdbLeaf(w.focusedLeaf()) {
		_ = tab.ReplaceFocusedWidget(logo)
		tab.SetLeafMark(leafMarkCode, tab.FindLeaf(isCodeSlot))
		return
	}
	if tab.ReplaceMatchingLeafWidget(logo, isCodeSlot) {
		tab.SetLeafMark(leafMarkCode, tab.FindLeaf(isCodeSlot))
	}
}

// swapFocusedWidget replaces the focused pane's widget and pushes the previous
// one onto the jump list (for Ctrl-O). Refuses when the focused leaf is the
// fixed GDB layout slot and wid is not gdbWidget.
func (w *Workspace) swapFocusedWidget(wid termui.Widget) bool {
	tab := w.Tab()
	if tab == nil || wid == nil || w.app == nil {
		return false
	}
	if w.isGdbLeaf(w.focusedLeaf()) && wid != w.app.gdbWidget {
		return false
	}
	prev := w.app.focusedWidget()
	if prev == wid {
		return false
	}
	if !tab.ReplaceFocusedWidget(wid) {
		return false
	}
	if prev != nil {
		w.pushWidgetJump(prev)
	}
	w.rememberCodeLeafFromFocus()
	return true
}

func (w *Workspace) pushWidgetJump(wid termui.Widget) {
	if wid == nil {
		return
	}
	// Avoid consecutive duplicates.
	if n := len(w.widgetJump); n > 0 && w.widgetJump[n-1] == wid {
		return
	}
	w.widgetJump = append(w.widgetJump, wid)
	if len(w.widgetJump) > widgetJumpMax {
		w.widgetJump = w.widgetJump[len(w.widgetJump)-widgetJumpMax:]
	}
}

// JumpBack restores the previous widget in the focused pane (Vim Ctrl-O).
// Leaves the jump stack untouched when the focused leaf is the GDB slot and
// the restore target is not gdbWidget.
func (w *Workspace) JumpBack(args ...any) {
	tab := w.Tab()
	if tab == nil || w.app == nil || len(w.widgetJump) == 0 {
		return
	}
	prev := w.widgetJump[len(w.widgetJump)-1]
	if w.isGdbLeaf(w.focusedLeaf()) && prev != w.app.gdbWidget {
		return
	}
	w.widgetJump = w.widgetJump[:len(w.widgetJump)-1]
	if tab.ReplaceFocusedWidget(prev) {
		w.app.RequestFrame()
	}
}
