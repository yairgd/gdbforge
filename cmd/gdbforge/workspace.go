package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/termui"
)

// Named leaf marks on the active WidgetTree (workspace role names).
const (
	leafMarkCode = "code"
	leafMarkGDB  = "gdb"
	leafMarkAsm  = "asm"
	// leafMarkLast is the Esc restore target when the user last focused a pane
	// that is neither Code nor GDB (breakpoints, callstack, …). Focusing Code
	// clears it; focusing GDB leaves it unchanged.
	leafMarkLast = "last"
)

const widgetJumpMax = 32

// LayoutShell owns gdbforge workspace policy above a termui.TabWidget:
// pane marks, placement, focus activation (Code/GDB/last), layout apply, and
// focused-pane widget swap / jump-back.
//
// It does not own debugger domain state (breakpoints, stops, threads, …).
// Generic tree operations stay on TabWidget — callers use Tab().
//
// Assumption (current): the Tab hosts a WidgetTree. If Tab later hosts other
// content types, gdbforge LayoutShell stays the split-tree policy layer; other
// tab contents would use different app policy, not these mark/slot APIs.
type LayoutShell struct {
	tab        *termui.TabWidget
	host       layoutHost
	widgetJump []termui.Widget
}

func initLayoutShell(app *DebuggerApp, tab *termui.TabWidget) {
	if app == nil {
		return
	}
	app.tab = tab
	app.host = app
}

// Tab returns the underlying generic TabWidget for tree operations
// (focus navigation, splits, HandleEvent, …).
func (w *LayoutShell) Tab() *termui.TabWidget {
	if w == nil {
		return nil
	}
	return w.tab
}

// Widget returns the TabWidget as a termui.Widget for TermApp.AddWidget.
func (w *LayoutShell) Widget() termui.Widget {
	if w == nil {
		return nil
	}
	return w.tab
}

func (w *LayoutShell) setTab(tab *termui.TabWidget) {
	if w == nil {
		return
	}
	w.tab = tab
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
