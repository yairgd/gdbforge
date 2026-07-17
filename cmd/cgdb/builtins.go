package main

import (
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/termui"
)

// initBuiltins creates singleton built-in views once at startup.
// Adding a new page: construct it here, registerBuiltin(name, w), and add
// Cmd(name, a.showBuiltin(name)) under Group("edit") in ExapData.
func (a *DebuggerApp) initBuiltins(outputChan <-chan core.GdbOutputMsg) {
	a.builtins = make(map[string]termui.Widget)

	a.aboutWidget = widgets.NewAboutWidget()
	a.registerBuiltin("about", a.aboutWidget)

	a.gdbWidget = widgets.NewGDBWidget(a.gdbClient)
	a.gdbWidget.SetClipboard(a.ClipboardIO())
	a.gdbWidget.StartGdbUIBridge(a.Screen(), outputChan)
	a.registerBuiltin("gdb", a.gdbWidget)
}

func (a *DebuggerApp) registerBuiltin(name string, w termui.Widget) {
	if a.builtins == nil {
		a.builtins = make(map[string]termui.Widget)
	}
	a.builtins[name] = w
}

// showBuiltin returns a command action that replaces the focused window's
// widget with the named singleton. No split, no new instance, no disk I/O.
func (a *DebuggerApp) showBuiltin(name string) func(args ...any) {
	return func(args ...any) {
		w := a.builtins[name]
		if w == nil || a.tab == nil {
			return
		}
		if a.swapFocusedWidget(w) {
			a.RequestFrame()
		}
	}
}

const widgetJumpMax = 32

// swapFocusedWidget replaces the focused pane's widget and pushes the previous
// one onto the jump list (for Ctrl-O).
func (a *DebuggerApp) swapFocusedWidget(w termui.Widget) bool {
	if a.tab == nil || w == nil {
		return false
	}
	prev := a.tab.FocusedWidget()
	if prev == w {
		return false
	}
	if !a.tab.ReplaceFocusedWidget(w) {
		return false
	}
	if prev != nil {
		a.pushWidgetJump(prev)
	}
	return true
}

func (a *DebuggerApp) pushWidgetJump(w termui.Widget) {
	if w == nil {
		return
	}
	// Avoid consecutive duplicates.
	if n := len(a.widgetJump); n > 0 && a.widgetJump[n-1] == w {
		return
	}
	a.widgetJump = append(a.widgetJump, w)
	if len(a.widgetJump) > widgetJumpMax {
		a.widgetJump = a.widgetJump[len(a.widgetJump)-widgetJumpMax:]
	}
}

// JumpBack restores the previous widget in the focused pane (Vim Ctrl-O).
func (a *DebuggerApp) JumpBack(args ...any) {
	if a.tab == nil || len(a.widgetJump) == 0 {
		return
	}
	prev := a.widgetJump[len(a.widgetJump)-1]
	a.widgetJump = a.widgetJump[:len(a.widgetJump)-1]
	if a.tab.ReplaceFocusedWidget(prev) {
		a.RequestFrame()
	}
}
