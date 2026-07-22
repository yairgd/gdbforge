package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/mcp"
)

// syncBreakpointViews pushes the shared BreakpointList to BP + Code views and
// snapshots the list for .gdbforge/breakpoints.yaml on quit.
func (a *DebuggerApp) syncBreakpointViews() {
	if a.breakpoints == nil {
		return
	}
	items := a.breakpoints.Items()
	if a.bpWidget != nil {
		a.bpWidget.SetItems(items)
	}
	a.paintCodeBreakmarks(items)
	a.bpSnapshot = items
	a.bpSnapshotSet = true
}

func (a *DebuggerApp) sendBreakpointCmd(cmd string) {
	if cmd == "" {
		return
	}
	if a.isDLV() {
		cmd = dlv.MapBreakCmd(cmd)
	}
	gdb.SendCmd(a.GDB(), a.State(), cmd)
	a.onBreakpointsChanged()
}

// restoreSavedBreakpoints reloads ./.gdbforge/breakpoints.yaml into GDB + UI.
func (a *DebuggerApp) restoreSavedBreakpoints(saved []mcp.BreakInfo) {
	if a == nil || len(saved) == 0 || a.breakpoints == nil {
		return
	}
	sess := a.GDB()
	if sess == nil {
		return
	}
	st := a.State()

	// Merge current GDB BPs first (e.g. from -x) so we do not duplicate.
	if items, ok := a.fetchBreakInfos(); ok {
		a.applyBreakInfos(items)
	}

	for _, it := range saved {
		if it.File == "" || it.Line < 1 {
			continue
		}
		if a.breakpoints.IndexOfFileLine(it.File, it.Line) >= 0 {
			continue
		}
		gdb.SendCmd(sess, st, breakInsertCmd(it.File, it.Line))
	}

	items, ok := a.fetchBreakInfos()
	if !ok {
		// Still seed the model so the BP pane shows saved rows until refresh.
		a.applyBreakInfos(saved)
		return
	}
	a.applyBreakInfos(items)

	// Apply disabled flags from the saved file.
	for _, want := range saved {
		if want.Enabled {
			continue
		}
		idx := a.breakpoints.IndexOfFileLine(want.File, want.Line)
		if idx < 0 {
			continue
		}
		cur, ok := a.breakpoints.At(idx)
		if !ok || !cur.Enabled || cur.Number < 1 {
			continue
		}
		gdb.SendCmd(sess, st, fmt.Sprintf("disable %d", cur.Number))
	}
	a.onBreakpointsChanged()
}

func breakInsertCmd(file string, line int) string {
	loc := fmt.Sprintf("%s:%d", file, line)
	if strings.ContainsAny(file, " \t\"") {
		base := filepath.Base(file)
		loc = fmt.Sprintf("%s:%d", base, line)
	}
	return "break " + loc
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
