package main

import (
	"fmt"
	"strings"

	"path/filepath"

	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
)

// syncBreakpointViews pushes the shared BreakpointList to BP + Code + Asm views and
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
	a.paintAsmBreakmarks(items)
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
	gdb.SendCmd(a.GDB(), a.State(), a.Debug(), cmd)
	if a.isDLV() {
		// Do not Query("breakpoints") immediately — after exit Delve may ask
		// [Y/n]? and the query line would be consumed as the answer.
		a.dlvBPDeferred = true
		return
	}
	a.onBreakpointsChanged()
}

// restoreSavedBreakpoints reloads ./.gdbforge/breakpoints.yaml into GDB + UI.
func (a *DebuggerApp) restoreSavedBreakpoints(saved []models.BreakInfo) {
	if a == nil || len(saved) == 0 || a.breakpoints == nil {
		return
	}
	sess := a.GDB()
	if sess == nil {
		return
	}

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
		gdb.SendCmd(sess, a.State(), a.Debug(), breakInsertCmd(it.File, it.Line))
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
		gdb.SendCmd(sess, a.State(), a.Debug(), fmt.Sprintf("disable %d", cur.Number))
	}

	// Re-apply conditions from the saved file (after numbers are known).
	items, ok = a.fetchBreakInfos()
	if ok {
		a.applyBreakInfos(items)
	}
	for _, want := range saved {
		if want.Condition == "" {
			continue
		}
		idx := a.breakpoints.IndexOfFileLine(want.File, want.Line)
		if idx < 0 {
			continue
		}
		cur, ok := a.breakpoints.At(idx)
		if !ok || cur.Number < 1 {
			continue
		}
		if cur.Condition == want.Condition {
			continue
		}
		gdb.SendCmd(sess, a.State(), a.Debug(), fmt.Sprintf("condition %d %s", cur.Number, want.Condition))
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
	prev, _ := a.breakpoints.At(index)
	cond := prev.Condition
	cmd, ok := a.breakpoints.ToggleEnableAt(index)
	if !ok {
		return
	}
	a.syncBreakpointViews()
	if a.bpWidget != nil {
		a.bpWidget.SelectIndex(index)
	}
	a.sendBreakpointCmd(cmd)
	// Re-enable only inserts the break; restore condition after GDB assigns a number.
	if strings.HasPrefix(cmd, "break ") && cond != "" {
		it, ok := a.breakpoints.At(index)
		if ok {
			a.reapplyConditionAfterEnable(cmd, it.File, it.Line, it.Addr, cond)
		}
	}
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

func (a *DebuggerApp) onAsmBreakToggle(addr string) {
	if a.breakpoints == nil {
		return
	}
	addr = parse.NormalizeAddr(addr)
	if addr == "" {
		return
	}
	cmd, ok := a.breakpoints.ToggleInsertClearAddr(addr)
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
	prevCond := ""
	if idx := a.breakpoints.IndexOfFileLine(path, line); idx >= 0 {
		if it, ok := a.breakpoints.At(idx); ok {
			prevCond = it.Condition
		}
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
	a.reapplyConditionAfterEnable(cmd, path, line, "", prevCond)
}

func (a *DebuggerApp) toggleAsmBreakEnable() {
	if a.assemblyWidget == nil || a.breakpoints == nil {
		return
	}
	addr := parse.NormalizeAddr(a.assemblyWidget.SelAddr())
	if addr == "" {
		return
	}
	prevCond := ""
	if idx := a.breakpoints.IndexOfAddr(addr); idx >= 0 {
		if it, ok := a.breakpoints.At(idx); ok {
			prevCond = it.Condition
		}
	}
	cmd, idx, ok := a.breakpoints.ToggleEnableAtAddr(addr, a.assemblyWidget.HasEnabledBreak(addr))
	if !ok {
		return
	}
	a.syncBreakpointViews()
	if a.bpWidget != nil && idx >= 0 {
		a.bpWidget.SelectIndex(idx)
	}
	a.sendBreakpointCmd(cmd)
	a.reapplyConditionAfterEnable(cmd, "", 0, addr, prevCond)
}

// reapplyConditionAfterEnable sends condition N expr after a re-enable break insert.
func (a *DebuggerApp) reapplyConditionAfterEnable(cmd, file string, line int, addr, cond string) {
	if !strings.HasPrefix(cmd, "break ") || cond == "" || a.breakpoints == nil {
		return
	}
	var cur models.BreakInfo
	var ok bool
	switch {
	case file != "" && line > 0:
		idx := a.breakpoints.IndexOfFileLine(file, line)
		if idx < 0 {
			return
		}
		cur, ok = a.breakpoints.At(idx)
	case addr != "":
		idx := a.breakpoints.IndexOfAddr(addr)
		if idx < 0 {
			return
		}
		cur, ok = a.breakpoints.At(idx)
	default:
		return
	}
	if !ok || cur.Number < 1 {
		return
	}
	a.sendBreakpointCmd(fmt.Sprintf("condition %d %s", cur.Number, cond))
}

// applyBreakInfos merges GDB -break-list into the shared model and syncs views.
func (a *DebuggerApp) applyBreakInfos(gdbItems []models.BreakInfo) {
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

// paintAsmBreakmarks applies address breakpoints to the AssemblyWidget.
func (a *DebuggerApp) paintAsmBreakmarks(items []models.BreakInfo) {
	if a.assemblyWidget == nil {
		return
	}
	if items == nil {
		items = []models.BreakInfo{}
	}
	a.assemblyWidget.SetBreakInfos(items)
}
