package main

import "github.com/yairgd/gdbforge/internal/termui"

// onCompletionMsg applies Tab results to the CompletionMenu and syncs the view.
func (a *DebuggerApp) onCompletionMsg(msg termui.CompletionMsg) {
	if a.completionMenu == nil {
		return
	}
	a.completionMenu.Set(msg.Names)
	a.syncCompletionView()
}

func (a *DebuggerApp) syncCompletionView() {
	if a.completionView == nil {
		return
	}
	if a.completionMenu == nil || !a.completionMenu.Active() {
		a.completionView.Clear()
		return
	}
	names, sel := a.completionMenu.Snapshot()
	a.completionView.SetItems(names, sel)
}

func (a *DebuggerApp) clearCompletion() {
	if a.completionMenu != nil {
		a.completionMenu.Clear()
	}
	if a.completionView != nil {
		a.completionView.Clear()
	}
}

func (a *DebuggerApp) completionActive() bool {
	return a.completionMenu != nil && a.completionMenu.Active()
}
