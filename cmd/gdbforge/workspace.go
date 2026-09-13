package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/termui"
)

// Named leaf marks on the active SplitLayout (workspace role names).
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

// LayoutShell owns gdbforge workspace policy above a termui.SplitLayout:
// pane marks, placement, focus activation (Code/GDB/last), layout apply, and
// focused-pane widget swap / jump-back.
//
// It does not own debugger domain state (breakpoints, stops, threads, …).
// Generic pane operations stay on the layout — callers use Layout(), which
// returns it concretely so nothing here forwards.
//
// LayoutShell is the split-tree policy layer specifically. A tab hosting some
// other termui.Layout needs its own policy, not these mark and slot APIs;
// Layout() returns nil in that case.
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

// Tab returns the tab container. It only hosts the layout — use Layout for
// pane, focus and mark operations.
func (w *LayoutShell) Tab() *termui.TabWidget {
	if w == nil {
		return nil
	}
	return w.tab
}

// Layout returns the active split layout for direct pane, focus and mark
// operations. Typed concretely so callers need no assertion and Tab needs no
// forwarding methods.
//
// Returns nil before the shell is wired and when the tab hosts a non-split
// Layout. Callers must nil-check: SplitLayout embeds *WidgetTree, so calling a
// promoted method on a nil *SplitLayout panics when the embedded field is read,
// before any nil receiver check inside WidgetTree can run.
func (w *LayoutShell) Layout() *termui.SplitLayout {
	if w == nil || w.tab == nil {
		return nil
	}
	lay, _ := w.tab.Layout().(*termui.SplitLayout)
	return lay
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
