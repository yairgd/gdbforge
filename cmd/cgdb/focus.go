package main

import (
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/termui"
)

// focusedWidget returns the focused leaf widget via Tab (generic shell only).
// Debugger-specific type knowledge lives in the helpers below, not at call sites.
func (a *DebuggerApp) focusedWidget() termui.Widget {
	if a.tab == nil {
		return nil
	}
	return a.tab.FocusedWidget()
}

// focusedCode returns the focused pane as a CodeWidget, or nil.
func (a *DebuggerApp) focusedCode() *widgets.CodeWidget {
	cw, ok := a.focusedWidget().(*widgets.CodeWidget)
	if !ok || cw == nil {
		return nil
	}
	return cw
}

// focusedBreakpoint returns the focused pane as a BreakpointWidget, or nil.
func (a *DebuggerApp) focusedBreakpoint() *widgets.BreakpointWidget {
	bp, ok := a.focusedWidget().(*widgets.BreakpointWidget)
	if !ok || bp == nil {
		return nil
	}
	return bp
}

// focusedIsBreakpoint reports whether the Breakpoint list pane has focus.
func (a *DebuggerApp) focusedIsBreakpoint() bool {
	return a.focusedBreakpoint() != nil
}

// focusedIsCode reports whether a CodeWidget pane has focus.
func (a *DebuggerApp) focusedIsCode() bool {
	return a.focusedCode() != nil
}
