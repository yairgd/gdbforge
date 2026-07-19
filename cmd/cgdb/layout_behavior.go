package main

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/cgdb/layout"
)

// layoutBehavior is per-layout normal-mode key policy (geometry stays in
// internal/cgdb/layout; this stays private to DebuggerApp).
type layoutBehavior interface {
	HandleNormalKey(a *DebuggerApp, ev *tcell.EventKey) bool
}

// standardDebugKeys is shared by default / panels / classic: Up/Down/Space go
// to Code only when focus is Code or GDB; list panes keep their own navigation;
// e on the Breakpoint pane stays local; n/s always step.
type standardDebugKeys struct{}

func (standardDebugKeys) HandleNormalKey(a *DebuggerApp, ev *tcell.EventKey) bool {
	return a.handleCodeGlobalKey(ev)
}

func (a *DebuggerApp) currentLayoutBehavior() layoutBehavior {
	switch a.State().CurrentLayout() {
	case layout.Default, layout.Panels, layout.Classic:
		return standardDebugKeys{}
	default:
		return standardDebugKeys{}
	}
}
