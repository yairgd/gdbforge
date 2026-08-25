package main

import (
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
)

// dlvHost is the narrow surface dlvCtl needs from the composition root.
type dlvHost interface {
	RequestFrame()
	DebugInfoSelectedLevel() int
	ScheduleDebugInfoRefresh()
	ScheduleStackRefresh()
	DebugInfoSelectLevel(level int)
	DebugInfoSyncCallStackViews()
	UpdateCodeAfterStop(stop *gdb.MiStopMsg) *widgets.CodeWidget
	PaintBreakpointMarks()
	ShowFrameSource(fr models.StackFrame)
	PresentLocation(codeW *widgets.CodeWidget, fr *models.StackFrame)
}

func (a *DebuggerApp) DebugInfoSelectedLevel() int {
	if a == nil {
		return 0
	}
	return a.debugInfo.selectedLevel()
}

func (a *DebuggerApp) ScheduleDebugInfoRefresh() {
	if a != nil {
		a.debugInfo.scheduleRefresh()
	}
}

func (a *DebuggerApp) ScheduleStackRefresh() {
	if a != nil {
		a.debugInfo.scheduleStackRefresh()
	}
}

func (a *DebuggerApp) DebugInfoSelectLevel(level int) {
	if a != nil {
		a.debugInfo.selectLevel(level)
	}
}

func (a *DebuggerApp) DebugInfoSyncCallStackViews() {
	if a != nil {
		a.debugInfo.syncCallStackViews()
	}
}

func (a *DebuggerApp) UpdateCodeAfterStop(stop *gdb.MiStopMsg) *widgets.CodeWidget {
	return a.updateCodeAfterStop(stop)
}

func (a *DebuggerApp) PaintBreakpointMarks() {
	if a == nil || a.breaks.List() == nil {
		return
	}
	a.breaks.paintCodeMarks(a.breaks.Items())
}

func (a *DebuggerApp) ShowFrameSource(fr models.StackFrame) { a.showFrameSource(fr) }

func (a *DebuggerApp) PresentLocation(codeW *widgets.CodeWidget, fr *models.StackFrame) {
	a.presentLocation(codeW, fr)
}
