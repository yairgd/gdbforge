package main

import (
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/mcp"
)

// syncBreakpointViews pushes the shared BreakpointList to BP + Code views.
func (a *DebuggerApp) syncBreakpointViews() {
	if a.breakpoints == nil {
		return
	}
	items := a.breakpoints.Items()
	if a.bpWidget != nil {
		a.bpWidget.SetItems(items)
	}
	a.paintCodeBreakmarks(items)
}

func (a *DebuggerApp) sendBreakpointCmd(cmd string) {
	if cmd == "" {
		return
	}
	gdb.SendCmd(a.GDB(), a.State(), cmd)
	a.onBreakpointsChanged()
}

func (a *DebuggerApp) onBreakpointToggle(index int) {
	if a.breakpoints == nil {
		return
	}
	cmd, ok := a.breakpoints.ToggleEnableAt(index)
	if !ok {
		return
	}
	a.syncBreakpointViews()
	if a.bpWidget != nil {
		a.bpWidget.SelectIndex(index)
	}
	a.sendBreakpointCmd(cmd)
}

func (a *DebuggerApp) onBreakpointDelete(index int) {
	if a.breakpoints == nil {
		return
	}
	cmd, ok := a.breakpoints.DeleteAt(index)
	if !ok {
		return
	}
	a.syncBreakpointViews()
	a.sendBreakpointCmd(cmd)
}

func (a *DebuggerApp) onCodeBreakToggle(path string, line int) {
	if a.breakpoints == nil || path == "" || line < 1 {
		return
	}
	cmd, ok := a.breakpoints.ToggleInsertClear(path, line)
	if !ok {
		return
	}
	a.syncBreakpointViews()
	a.sendBreakpointCmd(cmd)
}

func (a *DebuggerApp) toggleCodeBreakEnableOn(cw *widgets.CodeWidget) {
	if cw == nil || a.breakpoints == nil {
		return
	}
	path := cw.Path()
	line := cw.SelLine()
	if path == "" || line < 1 {
		return
	}
	cmd, idx, ok := a.breakpoints.ToggleEnableAtFileLine(path, line, cw.HasEnabledBreak(line))
	if !ok {
		return
	}
	a.syncBreakpointViews()
	if a.bpWidget != nil && idx >= 0 {
		a.bpWidget.SelectIndex(idx)
	}
	a.sendBreakpointCmd(cmd)
}

// applyBreakInfos merges GDB -break-list into the shared model and syncs views.
func (a *DebuggerApp) applyBreakInfos(gdbItems []mcp.BreakInfo) {
	if a.breakpoints == nil {
		a.breakpoints = &models.BreakpointList{}
	}
	a.breakpoints.MergeFromGDB(gdbItems)
	a.syncBreakpointViews()
}

// paintCodeWidgetBreaks applies the shared BP list to one CodeWidget.
func (a *DebuggerApp) paintCodeWidgetBreaks(w *widgets.CodeWidget, path string) {
	if w == nil || a.breakpoints == nil {
		return
	}
	w.SetBreakInfos(breaksForFile(a.breakpoints.Items(), path))
}
