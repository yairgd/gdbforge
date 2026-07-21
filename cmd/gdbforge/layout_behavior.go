package main

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/layout"
)

// layoutBehavior is per-layout normal-mode key policy (geometry stays in
// internal/gdbforge/layout; this stays private to DebuggerApp).
type layoutBehavior interface {
	HandleNormalKey(a *DebuggerApp, ev *tcell.EventKey) bool
}

// standardDebugKeys: debug keys live on the normal-mode key trie
// (InitKeyBindings). Hook kept for future layout-specific overlays.
type standardDebugKeys struct{}

func (standardDebugKeys) HandleNormalKey(a *DebuggerApp, ev *tcell.EventKey) bool {
	return false
}

func (a *DebuggerApp) currentLayoutBehavior() layoutBehavior {
	switch a.State().CurrentLayout() {
	case layout.Default, layout.Panels, layout.Classic:
		return standardDebugKeys{}
	default:
		return standardDebugKeys{}
	}
}
