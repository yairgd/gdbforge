package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
)

// syncThreadViews pushes the shared ThreadList to the Threads view.
func (a *DebuggerApp) syncThreadViews() {
	if a.threads == nil || a.threadWidget == nil {
		return
	}
	a.threadWidget.SetItems(a.threads.Items())
}

// syncCallStackViews pushes the shared CallStack to the Call Stack view.
func (a *DebuggerApp) syncCallStackViews() {
	if a.callstack == nil || a.callstackWidget == nil {
		return
	}
	a.callstackWidget.SetItems(a.callstack.Items())
}

// syncFileListViews pushes AppState source files to the FileList view.
func (a *DebuggerApp) syncFileListViews() {
	if a.fileListWidget == nil {
		return
	}
	if files := a.Debug().SourceFiles(); len(files) > 0 {
		a.fileListWidget.SetItems(files)
	}
}

func (a *DebuggerApp) applyThreadInfos(items []models.ThreadInfo) {
	a.setThreadInfos(items)
	a.syncThreadViews()
}

func (a *DebuggerApp) applyStackFrames(frames []models.StackFrame) {
	a.setStackFrames(frames)
	a.syncCallStackViews()
}

func (a *DebuggerApp) setThreadInfos(items []models.ThreadInfo) {
	if a.threads == nil {
		a.threads = &models.ThreadList{}
	}
	a.threads.Set(items)
}

func (a *DebuggerApp) setStackFrames(frames []models.StackFrame) {
	if a.callstack == nil {
		a.callstack = &models.CallStack{}
	}
	a.callstack.Set(frames)
}

// clearDebugInfoPanes resets Threads, Call Stack, Code pane, and breakpoints
// after the inferior exits (kill / exit).
func (a *DebuggerApp) clearDebugInfoPanes() {
	a.setThreadInfos(nil)
	a.setStackFrames(nil)
	a.syncThreadViews()
	a.syncCallStackViews()
	a.clearCodePane()
	a.clearBreakpointViews()
}

// clearBreakpointViews empties the shared BP model and gutters (UI only).
// Does not clear bpSnapshot — that is saved on quit after kill/exit reset.
func (a *DebuggerApp) clearBreakpointViews() {
	if a.breakpoints == nil {
		a.breakpoints = &models.BreakpointList{}
	} else {
		a.breakpoints.Clear()
	}
	if a.bpWidget != nil {
		a.bpWidget.SetItems(nil)
	}
	a.paintCodeBreakmarks(nil)
}

// clearCodePane empties Code widgets and restores the logo splash in the code leaf.
func (a *DebuggerApp) clearCodePane() {
	seen := make(map[*widgets.CodeWidget]bool)
	for _, w := range a.fileBuffers {
		if w == nil {
			continue
		}
		w.Clear()
		seen[w] = true
	}
	if a.primaryCode != nil && !seen[a.primaryCode] {
		a.primaryCode.Clear()
	}
	a.primaryCode = nil
	if a.State() != nil {
		a.Debug().SetCurrentLocation("", 0)
		a.Debug().ClearStopLocation()
	}
	a.placeLogoInCodeSlot()
}

// placeLogoInCodeSlot puts the startup logo back in the code leaf (after kill).
func (a *DebuggerApp) placeLogoInCodeSlot() {
	if a.tab == nil {
		return
	}
	logo := a.logoWidget
	if logo == nil {
		logo = widgets.NewLogoWidget()
		a.logoWidget = logo
	}
	if _, ok := a.focusedWidget().(*widgets.CodeWidget); ok {
		_ = a.tab.ReplaceFocusedWidget(logo)
		a.tab.SetLeafMark(leafMarkCode, a.tab.FindLeaf(isCodeSlot))
		return
	}
	if a.tab.ReplaceMatchingLeafWidget(logo, isCodeSlot) {
		a.tab.SetLeafMark(leafMarkCode, a.tab.FindLeaf(isCodeSlot))
	}
}
