package main

import (
	"context"
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
	"github.com/yairgd/gdbforge/internal/gdbforge/persist"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
)

// List returns the shared BreakpointList (may be nil before InitB).
func (c *breakCtl) List() *models.BreakpointList { return c.list }

// Items returns a copy of current breakpoint rows (nil-safe).
func (c *breakCtl) Items() []models.BreakInfo {
	if c.list == nil {
		return nil
	}
	return c.list.Items()
}

func (c *breakCtl) ensureList() *models.BreakpointList {
	if c.list == nil {
		c.list = &models.BreakpointList{}
	}
	return c.list
}

// clearModel empties the shared BP model and gutters (UI only).
// Does not clear snapshot — that is saved on quit after kill/exit reset.
func (c *breakCtl) clearModel() {
	c.ensureList().Clear()
	h := c.host
	if h != nil && h.BPWidget() != nil {
		h.BPWidget().SetItems(nil)
	}
	c.paintCodeMarks(nil)
}

// syncViews pushes the shared BreakpointList to BP + Code + Asm views and
// snapshots the list for .gdbforge/breakpoints.yaml on quit.
func (c *breakCtl) syncViews() {
	if c.list == nil {
		return
	}
	items := c.list.Items()
	h := c.host
	if h != nil && h.BPWidget() != nil {
		h.BPWidget().SetItems(items)
	}
	c.paintCodeMarks(items)
	c.paintAsmMarks(items)
	c.snapshot = items
	c.snapshotSet = true
}

// syncBreakpointViews is the historical name used by callers outside this file.
func (c *breakCtl) syncBreakpointViews() { c.syncViews() }

func (c *breakCtl) cmdEnv() backend.CommandEnv {
	h := c.host
	if h == nil {
		return backend.CommandEnv{}
	}
	return backend.CommandEnv{
		Session:  h.Session(),
		App:      h.State(),
		Inferior: h.Debug(),
	}
}

func (c *breakCtl) applyBreakIntent(intent models.BreakIntent) {
	h := c.host
	if h == nil || h.Backend() == nil {
		return
	}
	be := h.Backend()
	env := c.cmdEnv()
	switch intent.Kind {
	case models.IntentInsert:
		if intent.Addr != "" {
			be.InsertBreakpointAddr(env, intent.Addr)
		} else {
			be.InsertBreakpoint(env, intent.File, intent.Line)
		}
	case models.IntentClear:
		if intent.Addr != "" {
			be.ClearBreakpointAddr(env, intent.Addr, intent.Number)
		} else {
			be.ClearBreakpointAt(env, intent.File, intent.Line, intent.Number)
		}
	case models.IntentDeleteByNumber:
		if intent.Addr != "" {
			be.ClearBreakpointAddr(env, intent.Addr, intent.Number)
		} else {
			be.ClearBreakpointAt(env, intent.File, intent.Line, intent.Number)
		}
	default:
		return
	}
	if be.BreakRefreshImmediate() {
		c.onChanged()
		return
	}
	// Do not Query("breakpoints") immediately — after exit Delve may ask
	// [Y/n]? and the query line would be consumed as the answer.
	h.DeferBPRefresh()
}

// sendBreakpointCmd applies a legacy MI/CLI string (prefer applyBreakIntent).
func (c *breakCtl) sendBreakpointCmd(cmd string) {
	h := c.host
	if h == nil || cmd == "" || h.Backend() == nil {
		return
	}
	h.Backend().SendMappedBreak(c.cmdEnv(), cmd)
	if h.Backend().BreakRefreshImmediate() {
		c.onChanged()
		return
	}
	h.DeferBPRefresh()
}

// restoreSaved reloads ./.gdbforge/breakpoints.yaml into GDB + UI.
func (c *breakCtl) restoreSaved(saved []models.BreakInfo) {
	h := c.host
	if h == nil || len(saved) == 0 {
		return
	}
	c.ensureList()
	if h.Backend() == nil {
		return
	}
	env := c.cmdEnv()

	// Merge current GDB BPs first (e.g. from -x) so we do not duplicate.
	if items, ok := c.fetchInfos(); ok {
		c.applyBreakInfos(items)
	}

	for _, it := range saved {
		if it.File == "" || it.Line < 1 {
			continue
		}
		if c.list.IndexOfFileLine(it.File, it.Line) >= 0 {
			continue
		}
		h.Backend().InsertBreakpoint(env, it.File, it.Line)
	}

	items, ok := c.fetchInfos()
	if !ok {
		// Still seed the model so the BP pane shows saved rows until refresh.
		c.applyBreakInfos(saved)
		return
	}
	c.applyBreakInfos(items)

	// Apply disabled flags from the saved file.
	for _, want := range saved {
		if want.Enabled {
			continue
		}
		idx := c.list.IndexOfFileLine(want.File, want.Line)
		if idx < 0 {
			continue
		}
		cur, ok := c.list.At(idx)
		if !ok || !cur.Enabled || cur.Number < 1 {
			continue
		}
		h.Backend().DisableBreakpoint(env, cur.Number)
	}

	// Re-apply conditions from the saved file (after numbers are known).
	items, ok = c.fetchInfos()
	if ok {
		c.applyBreakInfos(items)
	}
	for _, want := range saved {
		if want.Condition == "" {
			continue
		}
		idx := c.list.IndexOfFileLine(want.File, want.Line)
		if idx < 0 {
			continue
		}
		cur, ok := c.list.At(idx)
		if !ok || cur.Number < 1 {
			continue
		}
		if cur.Condition == want.Condition {
			continue
		}
		h.Backend().SetBreakpointCondition(env, cur.Number, want.Condition)
	}
	c.onChanged()
}

// Toggle enable/disable at the BreakpointWidget selection index.
func (c *breakCtl) Toggle(index int) {
	if c.list == nil {
		return
	}
	h := c.host
	prev, _ := c.list.At(index)
	cond := prev.Condition
	intent, ok := c.list.ToggleEnableAt(index)
	if !ok {
		return
	}
	c.syncViews()
	if h != nil && h.BPWidget() != nil {
		h.BPWidget().SelectIndex(index)
	}
	c.applyBreakIntent(intent)
	// Re-enable only inserts the break; restore condition after GDB assigns a number.
	if intent.Kind == models.IntentInsert && cond != "" {
		it, ok := c.list.At(index)
		if ok {
			c.reapplyConditionAfterEnable(true, it.File, it.Line, it.Addr, cond)
		}
	}
}

func (c *breakCtl) Delete(index int) {
	if c.list == nil {
		return
	}
	intent, ok := c.list.DeleteAt(index)
	if !ok {
		return
	}
	c.syncViews()
	c.applyBreakIntent(intent)
}

func (c *breakCtl) onCodeBreakToggle(path string, line int) {
	if c.list == nil || path == "" || line < 1 {
		return
	}
	intent, ok := c.list.ToggleInsertClear(path, line)
	if !ok {
		return
	}
	c.syncViews()
	c.applyBreakIntent(intent)
}

func (c *breakCtl) ToggleAsm(addr string) {
	if c.list == nil {
		return
	}
	addr = parse.NormalizeAddr(addr)
	if addr == "" {
		return
	}
	intent, ok := c.list.ToggleInsertClearAddr(addr)
	if !ok {
		return
	}
	c.syncViews()
	c.applyBreakIntent(intent)
}

func (c *breakCtl) toggleCodeBreakEnableOn(cw *widgets.CodeWidget) {
	if cw == nil || c.list == nil {
		return
	}
	h := c.host
	path := cw.Path()
	line := cw.SelLine()
	if path == "" || line < 1 {
		return
	}
	prevCond := ""
	if idx := c.list.IndexOfFileLine(path, line); idx >= 0 {
		if it, ok := c.list.At(idx); ok {
			prevCond = it.Condition
		}
	}
	intent, idx, ok := c.list.ToggleEnableAtFileLine(path, line, cw.HasEnabledBreak(line))
	if !ok {
		return
	}
	c.syncViews()
	if h != nil && h.BPWidget() != nil && idx >= 0 {
		h.BPWidget().SelectIndex(idx)
	}
	c.applyBreakIntent(intent)
	c.reapplyConditionAfterEnable(intent.Kind == models.IntentInsert, path, line, "", prevCond)
}

func (c *breakCtl) ToggleAsmEnable() {
	h := c.host
	if h == nil || h.AssemblyWidget() == nil || c.list == nil {
		return
	}
	aw := h.AssemblyWidget()
	addr := parse.NormalizeAddr(aw.SelAddr())
	if addr == "" {
		return
	}
	prevCond := ""
	if idx := c.list.IndexOfAddr(addr); idx >= 0 {
		if it, ok := c.list.At(idx); ok {
			prevCond = it.Condition
		}
	}
	intent, idx, ok := c.list.ToggleEnableAtAddr(addr, aw.HasEnabledBreak(addr))
	if !ok {
		return
	}
	c.syncViews()
	if h.BPWidget() != nil && idx >= 0 {
		h.BPWidget().SelectIndex(idx)
	}
	c.applyBreakIntent(intent)
	c.reapplyConditionAfterEnable(intent.Kind == models.IntentInsert, "", 0, addr, prevCond)
}

// reapplyConditionAfterEnable sends condition N expr after a re-enable break insert.
func (c *breakCtl) reapplyConditionAfterEnable(wasInsert bool, file string, line int, addr, cond string) {
	if !wasInsert || cond == "" || c.list == nil {
		return
	}
	var cur models.BreakInfo
	var ok bool
	switch {
	case file != "" && line > 0:
		idx := c.list.IndexOfFileLine(file, line)
		if idx < 0 {
			return
		}
		cur, ok = c.list.At(idx)
	case addr != "":
		idx := c.list.IndexOfAddr(addr)
		if idx < 0 {
			return
		}
		cur, ok = c.list.At(idx)
	default:
		return
	}
	if !ok || cur.Number < 1 {
		return
	}
	h := c.host
	if h == nil || h.Backend() == nil {
		return
	}
	h.Backend().SetBreakpointCondition(c.cmdEnv(), cur.Number, cond)
}

// applyBreakInfos merges GDB -break-list into the shared model and syncs views.
func (c *breakCtl) applyBreakInfos(gdbItems []models.BreakInfo) {
	c.ensureList().MergeFromGDB(gdbItems)
	c.syncViews()
}

// paintCodeWidget applies the shared BP list to one CodeWidget.
func (c *breakCtl) paintCodeWidget(w *widgets.CodeWidget, path string) {
	if w == nil || c.list == nil {
		return
	}
	w.SetBreakInfos(breaksForFile(c.list.Items(), path))
}

// paintAsmMarks applies address breakpoints to the AssemblyWidget.
func (c *breakCtl) paintAsmMarks(items []models.BreakInfo) {
	h := c.host
	if h == nil || h.AssemblyWidget() == nil {
		return
	}
	if items == nil {
		items = []models.BreakInfo{}
	}
	h.AssemblyWidget().SetBreakInfos(items)
}

// paintCodeMarks paints line-number gutters from AppState break colors.
func (c *breakCtl) paintCodeMarks(items []models.BreakInfo) {
	h := c.host
	if h == nil {
		return
	}
	seen := make(map[*widgets.CodeWidget]bool)
	for path, w := range h.FileBuffers() {
		if w == nil {
			continue
		}
		w.SetBreakInfos(breaksForFile(items, path))
		seen[w] = true
	}
	if pc := h.PrimaryCode(); pc != nil && !seen[pc] {
		if p := pc.Path(); p != "" {
			pc.SetBreakInfos(breaksForFile(items, p))
		}
	}
}

func (c *breakCtl) rebuildGutters() {
	h := c.host
	if h == nil {
		return
	}
	seen := make(map[*widgets.CodeWidget]bool)
	for _, w := range h.FileBuffers() {
		if w == nil {
			continue
		}
		w.RebuildBuffer()
		seen[w] = true
	}
	if pc := h.PrimaryCode(); pc != nil && !seen[pc] {
		pc.RebuildBuffer()
	}
}

func breaksForFile(items []models.BreakInfo, path string) []models.BreakInfo {
	out := make([]models.BreakInfo, 0)
	for _, it := range items {
		if models.SameSourcePath(it.File, path) {
			out = append(out, it)
		}
	}
	return out
}

func (c *breakCtl) fetchInfos() ([]models.BreakInfo, bool) {
	h := c.host
	if h == nil || h.GdbMcp() == nil || h.Backend() == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	items, ok := h.Backend().FetchBreakpoints(ctx, h.GdbMcp())
	return items, ok
}

// refresh queries GDB -break-list and merges into the shared model.
func (c *breakCtl) refresh() {
	items, ok := c.fetchInfos()
	if !ok {
		return
	}
	c.applyBreakInfos(items)
}

// refreshAfterStop re-queries live breakpoints after *stopped.
func (c *breakCtl) refreshAfterStop() {
	h := c.host
	if h == nil || h.GdbMcp() == nil {
		return
	}
	for attempt := 0; attempt < 6; attempt++ {
		if attempt > 0 {
			time.Sleep(40 * time.Millisecond)
		}
		items, ok := c.fetchInfos()
		if !ok {
			continue
		}
		c.applyBreakInfos(items)
		return
	}
}

func (c *breakCtl) runRefresh() {
	c.refresh()
	h := c.host
	if h != nil && h.Screen() != nil {
		_ = h.Screen().PostEvent(tcell.NewEventInterrupt(breakpointsUIMsg{}))
	}
}

// onChanged publishes a bus event; the Subscribe handler coalesces bursts.
func (c *breakCtl) onChanged() {
	if c.host == nil {
		return
	}
	c.host.PublishBreakpointsChanged()
}

func (c *breakCtl) onChangedMsg(_ BreakpointsChangedMsg) {
	h := c.host
	if h != nil && h.IsConfirming() {
		h.DeferBPRefresh()
		return
	}
	c.coalesce.Schedule(c.runRefresh)
}

func (c *breakCtl) Register(bus *platform.EventBus) {
	platform.Subscribe(bus, c.onChangedMsg)
	platform.Subscribe(bus, c.onBreakpointsUI)
	platform.Subscribe(bus, c.onCodeBreakToggleMsg)
	platform.Subscribe(bus, c.onCodeBreakEnableToggleMsg)
	platform.Subscribe(bus, c.onAsmBreakToggleMsg)
	platform.Subscribe(bus, c.onAsmBreakEnableToggleMsg)
	platform.Subscribe(bus, c.onBreakpointToggleMsg)
	platform.Subscribe(bus, c.onBreakpointDeleteMsg)
	platform.Subscribe(bus, c.onBreakpointActivateMsg)
}

func (c *breakCtl) onCodeBreakToggleMsg(msg events.CodeBreakToggleMsg) {
	c.onCodeBreakToggle(msg.Path, msg.Line)
}

func (c *breakCtl) onCodeBreakEnableToggleMsg(msg events.CodeBreakEnableToggleMsg) {
	path, line := msg.Path, msg.Line
	if path == "" || line < 1 {
		h := c.host
		if h == nil {
			return
		}
		cw := h.PrimaryCode()
		if cw == nil {
			return
		}
		path = cw.Path()
		line = cw.SelLine()
	}
	if path == "" || line < 1 {
		return
	}
	if h := c.host; h != nil {
		if cw := h.FileBuffers()[path]; cw != nil {
			c.toggleCodeBreakEnableOn(cw)
			return
		}
		for p, cw := range h.FileBuffers() {
			if p == path || cw.Path() == path {
				c.toggleCodeBreakEnableOn(cw)
				return
			}
		}
	}
}

func (c *breakCtl) onAsmBreakToggleMsg(msg events.AsmBreakToggleMsg) {
	c.ToggleAsm(msg.Addr)
}

func (c *breakCtl) onAsmBreakEnableToggleMsg(_ events.AsmBreakEnableToggleMsg) {
	c.ToggleAsmEnable()
}

func (c *breakCtl) onBreakpointToggleMsg(msg events.BreakpointToggleMsg) {
	c.Toggle(msg.Index)
}

func (c *breakCtl) onBreakpointDeleteMsg(msg events.BreakpointDeleteMsg) {
	c.Delete(msg.Index)
}

func (c *breakCtl) onBreakpointActivateMsg(msg events.BreakpointActivateMsg) {
	h := c.host
	if h == nil {
		return
	}
	h.ActivateBreakpoint(msg.BP)
	if msg.FocusCode {
		h.FocusCode()
	}
}

func (c *breakCtl) onBreakpointsUI(_ breakpointsUIMsg) {
	if c.list != nil {
		c.syncBreakpointViews()
	}
	if h := c.host; h != nil {
		h.RequestFrame()
	}
}

// maybeBreakMain inserts a default entry breakpoint when AppState.BreakMain is set.
func (c *breakCtl) maybeBreakMain() {
	h := c.host
	if h == nil || h.GDBWidget() == nil || !h.Debug().BreakMain() || h.Backend() == nil {
		return
	}
	h.Backend().InsertDefaultBreakMain(c.cmdEnv())
}

// reapplyAfterDlvConnect sets break main (if enabled) and re-inserts BPs.
func (c *breakCtl) reapplyAfterDlvConnect() {
	c.maybeBreakMain()
	if c.list == nil {
		return
	}
	for _, it := range c.list.Items() {
		if it.File == "" || it.Line < 1 {
			continue
		}
		c.applyBreakIntent(models.BreakIntent{Kind: models.IntentInsert, File: it.File, Line: it.Line})
	}
	c.onChanged()
}

func (c *breakCtl) hasAt(file string, line int) bool {
	if c.list == nil || file == "" || line < 1 {
		return false
	}
	return c.list.IndexOfFileLine(file, line) >= 0
}

// saveOnQuit writes snapshot to ./.gdbforge/breakpoints.yaml.
func (c *breakCtl) saveOnQuit() {
	if !c.snapshotSet {
		return
	}
	items := c.snapshot
	if c.list != nil {
		if cur := c.list.Items(); len(cur) > 0 {
			items = cur
		}
	}
	_ = persist.SaveBreakpoints(".", items)
}

// ActivateBreakpoint browses Code/Asm for a BP row — layout/focus orchestration.
func (a *DebuggerApp) ActivateBreakpoint(bp models.BreakInfo) {
	if a == nil {
		return
	}
	if bp.File == "" && bp.Addr != "" {
		aw := a.asm.Widget()
		if aw == nil || a.backend == nil || !a.backend.SupportsAssembly() {
			return
		}
		// Addr-only BP: show Asm in the location leaf (auto if not sticky).
		if !a.asm.hasSplit() && !a.asm.PreferAsm() {
			a.asm.setAutoAsm(true)
		}
		a.asm.placeInSlot(aw)
		if leaf := a.asm.findLeaf(); leaf != nil && a.asm.hasSplit() {
			_ = a.Tab().FocusLeaf(leaf)
		}
		go a.asm.runRefresh(bp.Addr, aw.VisibleRows(), false, aw.FuncName(), 0, -1)
		a.RequestFrame()
		return
	}
	if bp.File == "" {
		return
	}
	w := a.bufs.showCodeBrowse(bp.File, bp.Line)
	if w != nil && w.Unavailable() {
		w.ShowUnavailable(bp.File, formatUnavailableExtra("", bp.Line))
	}
	a.presentLocation(w, nil)
	if sourceUnavailable(w) && bp.Addr != "" && a.asm.AutoAsm() {
		if aw := a.asm.Widget(); aw != nil {
			go a.asm.runRefresh(bp.Addr, aw.VisibleRows(), false, aw.FuncName(), 0, -1)
		}
	}
	a.RequestFrame()
}
