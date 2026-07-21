package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/mcp"
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
	if files := a.State().SourceFiles(); len(files) > 0 {
		a.fileListWidget.SetItems(files)
	}
}

func (a *DebuggerApp) applyThreadInfos(items []mcp.ThreadInfo) {
	a.setThreadInfos(items)
	a.syncThreadViews()
}

func (a *DebuggerApp) applyStackFrames(frames []mcp.StackFrame) {
	a.setStackFrames(frames)
	a.syncCallStackViews()
}

func (a *DebuggerApp) setThreadInfos(items []mcp.ThreadInfo) {
	if a.threads == nil {
		a.threads = &models.ThreadList{}
	}
	a.threads.Set(items)
}

func (a *DebuggerApp) setStackFrames(frames []mcp.StackFrame) {
	if a.callstack == nil {
		a.callstack = &models.CallStack{}
	}
	a.callstack.Set(frames)
}
