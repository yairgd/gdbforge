package app

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/termforge"
)

// focusedWidget returns the focused leaf widget via Tab (generic shell only).
// Debugger-specific type knowledge lives in the helpers below, not at call sites.
func (a *DebuggerApp) focusedWidget() termforge.NodeWidget {
	if lay := a.Layout(); lay != nil {
		return lay.FocusedWidget()
	}
	return nil
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

// focusedCallstack returns the focused pane as a CallStackWidget, or nil.
func (a *DebuggerApp) focusedCallstack() *widgets.CallStackWidget {
	cs, ok := a.focusedWidget().(*widgets.CallStackWidget)
	if !ok || cs == nil {
		return nil
	}
	return cs
}

// focusedIsCallstack reports whether the Call Stack pane has focus.
func (a *DebuggerApp) focusedIsCallstack() bool {
	return a.focusedCallstack() != nil
}

// focusedIsGdb reports whether the GDB console pane has focus.
func (a *DebuggerApp) focusedIsGdb() bool {
	return a.gdbWidget != nil && a.focusedWidget() == a.gdbWidget
}

// focusedIsLuaConsole reports whether the Lua REPL pane has focus.
func (a *DebuggerApp) focusedIsLuaConsole() bool {
	return a.luaConsoleWidget != nil && a.focusedWidget() == a.luaConsoleWidget
}

// focusedTakesText reports whether the focused pane reads typed input (GDB or Lua console,
// inferior IO / exec terminal stdin) and so needs insert mode. Every other pane works from normal mode.
func (a *DebuggerApp) focusedTakesText() bool {
	if a.focusedIsGdb() || a.focusedIsLuaConsole() {
		return true
	}
	if a.outputWidget != nil && a.focusedWidget() == a.outputWidget {
		return true // IO pane: inferior stdin
	}
	return a.execWidget != nil && a.focusedWidget() == a.execWidget
}

// noteTextPane remembers the focused pane when it takes text, so 'i' can return to it.
func (a *DebuggerApp) noteTextPane() {
	if _, ok := a.focusedWidget().(*widgets.LuaWidget); ok || a.focusedTakesText() {
		a.lastTextPane = a.focusedWidget()
	}
}

// insertKey is 'i': type into the focused pane when it takes text, else return to the last
// text pane used (the GDB console until one was used).
func (a *DebuggerApp) insertKey() {
	a.noteTextPane()
	if lw, ok := a.focusedWidget().(*widgets.LuaWidget); ok {
		a.lua.enterMode(lw)
		return
	}
	if a.focusedTakesText() {
		a.EnterInsertMode()
		return
	}
	if last := a.lastTextPane; last != nil && last != termforge.NodeWidget(a.gdbWidget) {
		if lay := a.Layout(); lay != nil {
			if leaf := lay.FindLeaf(func(w termforge.Widget) bool { return w == last }); leaf != nil {
				a.rememberCodeLeafFromFocus()
				lay.FocusLeaf(leaf)
				a.insertKey()
				return
			}
		}
	}
	a.activateGdbInsertMode()
}
