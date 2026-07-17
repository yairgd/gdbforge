package main

import (
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/termui"
)

// initBuiltins creates singleton built-in views once at startup.
// Adding a new page: construct it here, registerBuiltin(name, w), and add
// Cmd(name, a.showBuiltin(name)) under Group("edit") in ExapData.
func (a *DebuggerApp) initBuiltins() {
	a.builtins = make(map[string]termui.Widget)
	a.aboutWidget = widgets.NewAboutWidget()
	a.registerBuiltin("about", a.aboutWidget)
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
		if a.tab.ReplaceFocusedWidget(w) {
			a.RequestFrame()
		}
	}
}
