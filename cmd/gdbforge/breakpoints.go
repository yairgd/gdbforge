package main

import (
	"fmt"
	"strings"

	"path/filepath"

	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
)

// syncBreakpointViews pushes the shared BreakpointList to BP + Code + Asm views and
// snapshots the list for .gdbforge/breakpoints.yaml on quit.
func (c *breakCtl) syncBreakpointViews() {
	a := c.app
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

func (c *breakCtl) sendBreakpointCmd(cmd string) {
	a := c.app
	if cmd == "" || a.backend == nil {
		return
	}
	cmd = a.backend.MapBreak(cmd)
	gdb.SendCmd(a.GDB(), a.State(), a.Debug(), cmd)
	if a.backend.BreakRefreshImmediate() {
		a.onBreakpointsChanged()
		return
	}
	// Do not Query("breakpoints") immediately — after exit Delve may ask
	// [Y/n]? and the query line would be consumed as the answer.
	a.dlvBPDeferred = true
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
		a.breaks.applyBreakInfos(items)
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
		a.breaks.applyBreakInfos(saved)
		return
	}
	a.breaks.applyBreakInfos(items)

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
		a.breaks.applyBreakInfos(items)
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

func (c *breakCtl) onBreakpointToggle(index int) {
	a := c.app
	if a.breakpoints == nil {
		return
	}
	prev, _ := a.breakpoints.At(index)
	cond := prev.Condition
	cmd, ok := a.breakpoints.ToggleEnableAt(index)
	if !ok {
		return
	}
	c.syncBreakpointViews()
	if a.bpWidget != nil {
		a.bpWidget.SelectIndex(index)
	}
	c.sendBreakpointCmd(cmd)
	// Re-enable only inserts the break; restore condition after GDB assigns a number.
	if strings.HasPrefix(cmd, "break ") && cond != "" {
		it, ok := a.breakpoints.At(index)
		if ok {
			a.breaks.reapplyConditionAfterEnable(cmd, it.File, it.Line, it.Addr, cond)
		}
	}
}

func (c *breakCtl) onBreakpointDelete(index int) {
	a := c.app
	if a.breakpoints == nil {
		return
	}
	cmd, ok := a.breakpoints.DeleteAt(index)
	if !ok {
		return
	}
	c.syncBreakpointViews()
	c.sendBreakpointCmd(cmd)
}

func (c *breakCtl) onCodeBreakToggle(path string, line int) {
	a := c.app
	if a.breakpoints == nil || path == "" || line < 1 {
		return
	}
	cmd, ok := a.breakpoints.ToggleInsertClear(path, line)
	if !ok {
		return
	}
	c.syncBreakpointViews()
	c.sendBreakpointCmd(cmd)
}

func (c *breakCtl) onAsmBreakToggle(addr string) {
	a := c.app
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
	c.syncBreakpointViews()
	c.sendBreakpointCmd(cmd)
}

func (c *breakCtl) toggleCodeBreakEnableOn(cw *widgets.CodeWidget) {
	a := c.app
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
	c.syncBreakpointViews()
	if a.bpWidget != nil && idx >= 0 {
		a.bpWidget.SelectIndex(idx)
	}
	c.sendBreakpointCmd(cmd)
	a.breaks.reapplyConditionAfterEnable(cmd, path, line, "", prevCond)
}

func (c *breakCtl) toggleAsmBreakEnable() {
	a := c.app
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
	c.syncBreakpointViews()
	if a.bpWidget != nil && idx >= 0 {
		a.bpWidget.SelectIndex(idx)
	}
	c.sendBreakpointCmd(cmd)
	a.breaks.reapplyConditionAfterEnable(cmd, "", 0, addr, prevCond)
}

// reapplyConditionAfterEnable sends condition N expr after a re-enable break insert.
func (c *breakCtl) reapplyConditionAfterEnable(cmd, file string, line int, addr, cond string) {
	a := c.app
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
	c.sendBreakpointCmd(fmt.Sprintf("condition %d %s", cur.Number, cond))
}

// applyBreakInfos merges GDB -break-list into the shared model and syncs views.
func (c *breakCtl) applyBreakInfos(gdbItems []models.BreakInfo) {
	a := c.app
	if a.breakpoints == nil {
		a.breakpoints = &models.BreakpointList{}
	}
	a.breakpoints.MergeFromGDB(gdbItems)
	c.syncBreakpointViews()
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


func (c *breakCtl) Activate(bp models.BreakInfo) {
	a := c.app
	if a == nil {
		return
	}
	if bp.File == "" && bp.Addr != "" {
		if a.assemblyWidget == nil || a.backend == nil || !a.backend.SupportsAssembly() {
			return
		}
		if a.hasAsmSplit() || a.preferAsm {
			a.placeAsmInSlot(a.assemblyWidget)
			if leaf := a.findAsmLeaf(); leaf != nil && a.hasAsmSplit() {
				_ = a.tab.FocusLeaf(leaf)
			}
			go a.runAssemblyRefresh(bp.Addr, a.assemblyWidget.VisibleRows(), false)
			a.RequestFrame()
		}
		return
	}
	if bp.File == "" {
		return
	}
	w := a.showCodeBrowse(bp.File, bp.Line)
	if w != nil && w.Unavailable() {
		w.ShowUnavailable(bp.File, formatUnavailableExtra("", bp.Line))
	}
	a.placeCodeInSlot(w)
	a.RequestFrame()
}
