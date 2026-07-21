package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
)

type codeRefreshMsg struct {
	widget *widgets.CodeWidget
}

type breakpointsUIMsg struct{}

type debugInfoUIMsg struct{}

func (a *DebuggerApp) maybeClearOutput() {
	if a.outputWidget == nil || !a.State().ClearOutput() {
		return
	}
	a.outputWidget.Clear()
}

// maybeBreakMain inserts "break main" when AppState.BreakMain is set (default on).
func (a *DebuggerApp) maybeBreakMain() {
	if a.gdbWidget == nil || !a.State().BreakMain() {
		return
	}
	sess := a.gdbWidget.Session()
	if sess == nil {
		return
	}
	widgets.SendGdbCmd(sess, a.State(), "break main")
}

// onGdbStopped updates AppState / CodeWidget when GDB hits a breakpoint or steps,
// and refreshes Threads / Call stack panes.
func (a *DebuggerApp) onGdbStopped(stop *gdb.MiStopMsg) {
	if stop == nil {
		return
	}
	file := stop.File
	line := stop.Line
	if file != "" {
		a.State().SetCurrentLocation(file, line)
	}

	a.scheduleDebugInfoRefresh()

	if file == "" {
		if stop.Func == "" {
			return
		}
		go func() {
			w := a.showCodeUnavailable(stop.Func, formatUnavailableExtra("", line))
			if scr := a.Screen(); scr != nil {
				_ = scr.PostEvent(tcell.NewEventInterrupt(codeRefreshMsg{widget: w}))
			}
		}()
		return
	}
	go func() {
		a.ensureSourceFiles()
		// Do not -break-list on every stop: that flooded the PTY and froze input.
		// Breakpoint list / red marks sync via =breakpoint-* notifies only.
		// Only paint gutters for a newly created buffer — re-painting an existing
		// CodeWidget from bpWidget here races Space toggles and stale MergeFromGDB.
		w := a.showCodeAt(file, line)
		if w != nil && w.Unavailable() {
			w.ShowUnavailable(file, formatUnavailableExtra(stop.Func, line))
		}
		if scr := a.Screen(); scr != nil {
			_ = scr.PostEvent(tcell.NewEventInterrupt(codeRefreshMsg{widget: w}))
		}
	}()
}

// showCodeAt loads file at line in a CodeWidget (━━▶) and paints BP gutters when new.
// Missing sources / shared libraries show a centered "not available" placeholder.
// Replaces the startup LogoWidget in the code leaf when present.
func (a *DebuggerApp) showCodeAt(file string, line int) *widgets.CodeWidget {
	if file == "" {
		return nil
	}
	w, created := a.ensureCodeBuffer(file)
	if w == nil {
		return nil
	}
	if line < 1 {
		line = 1
	}
	_ = w.ShowLocation(file, line)
	a.State().SetCurrentLocation(file, line)
	if created && !w.Unavailable() {
		a.paintCodeWidgetBreaks(w, file)
	}
	a.placeCodeInSlot(w)
	return w
}

// showCodeUnavailable shows a CodeWidget placeholder when there is no source path
// (e.g. ??? in libc) using func/detail as the displayed path line.
func (a *DebuggerApp) showCodeUnavailable(label, extra string) *widgets.CodeWidget {
	if label == "" {
		label = "(unknown)"
	}
	key := "unavailable:" + label
	w, _ := a.ensureCodeBuffer(key)
	if w == nil {
		return nil
	}
	w.ShowUnavailable(label, extra)
	w.PaneName = filepath.Base(label)
	if w.PaneName == "" || w.PaneName == "." {
		w.PaneName = "unavailable"
	}
	a.placeCodeInSlot(w)
	return w
}

// onCallStackActivate selects a stack frame in GDB and shows its source.
func (a *DebuggerApp) onCallStackActivate(fr mcp.StackFrame) {
	if a.gdbWidget == nil {
		return
	}
	sess := a.gdbWidget.Session()
	if sess == nil {
		return
	}
	widgets.SendGdbCmd(sess, a.State(), fmt.Sprintf("frame %d", fr.Level))
	a.showFrameSource(fr)
	a.RequestFrame()
}

// onGdbFrameSync refreshes Code / Call Stack after a GDB console frame/f/up/down
// (those do not emit *stopped).
func (a *DebuggerApp) onGdbFrameSync() {
	go func() {
		a.syncCurrentFrameFromGDB()
		if scr := a.Screen(); scr != nil {
			_ = scr.PostEvent(tcell.NewEventInterrupt(codeRefreshMsg{widget: a.activeCodeWidget()}))
		}
	}()
}

func (a *DebuggerApp) syncCurrentFrameFromGDB() {
	if a.gdbMcp == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	raw, err := a.gdbMcp.Query(ctx, "-stack-info-frame")
	if err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("frame").Error(err.Error())
		}
		return
	}
	fr, ok := mcp.ParseStackInfoFrame(raw)
	if !ok {
		return
	}

	// Keep the callstack list in sync and highlight the selected level.
	if a.callstackWidget != nil {
		rawStack, err := a.gdbMcp.Query(ctx, "-stack-list-frames")
		if err == nil {
			a.callstackWidget.SetItems(mcp.ParseStackListFrames(rawStack))
		}
		a.callstackWidget.SelectLevel(fr.Level)
	}

	a.showFrameSource(fr)
}

func (a *DebuggerApp) showFrameSource(fr mcp.StackFrame) {
	var w *widgets.CodeWidget
	switch {
	case fr.File != "":
		a.State().SetCurrentLocation(fr.File, fr.Line)
		w = a.showCodeAt(fr.File, fr.Line)
		if w != nil && w.Unavailable() {
			w.ShowUnavailable(fr.File, formatUnavailableExtra(fr.Func, fr.Line))
		}
	case fr.Func != "":
		w = a.showCodeUnavailable(fr.Func, formatUnavailableExtra("", fr.Line))
	default:
		return
	}
	if w != nil {
		a.applyCodeStop(w)
	}
}

// onBreakpointActivate shows the source at the selected breakpoint location.
func (a *DebuggerApp) onBreakpointActivate(bp mcp.BreakInfo) {
	if bp.File == "" {
		return
	}
	w := a.showCodeAt(bp.File, bp.Line)
	if w != nil && w.Unavailable() {
		w.ShowUnavailable(bp.File, formatUnavailableExtra("", bp.Line))
	}
	a.applyCodeStop(w)
	a.RequestFrame()
}

// onThreadActivate switches GDB to the selected thread, refreshes stack/threads,
// and shows the current frame source.
func (a *DebuggerApp) onThreadActivate(th mcp.ThreadInfo) {
	if a.gdbWidget == nil || th.ID == "" {
		return
	}
	sess := a.gdbWidget.Session()
	if sess == nil {
		return
	}
	widgets.SendGdbCmd(sess, a.State(), "thread "+th.ID)
	a.refreshThreadsAndStack()

	file, line := th.File, th.Line
	if a.callstackWidget != nil {
		if frames := a.callstackWidget.Items(); len(frames) > 0 {
			if frames[0].File != "" {
				file, line = frames[0].File, frames[0].Line
			}
		}
	}
	if file != "" {
		w := a.showCodeAt(file, line)
		if w != nil && w.Unavailable() {
			fn := th.Func
			if a.callstackWidget != nil {
				if frames := a.callstackWidget.Items(); len(frames) > 0 && frames[0].Func != "" {
					fn = frames[0].Func
				}
			}
			w.ShowUnavailable(file, formatUnavailableExtra(fn, line))
		}
		a.applyCodeStop(w)
	} else if th.Func != "" {
		w := a.showCodeUnavailable(th.Func, formatUnavailableExtra("", th.Line))
		a.applyCodeStop(w)
	}
	a.RequestFrame()
}

// formatUnavailableExtra builds the optional detail line under the path in
// CodeWidget's centered "not available" placeholder.
func formatUnavailableExtra(fn string, line int) string {
	switch {
	case fn != "" && line > 0:
		return fmt.Sprintf("%s  line %d", fn, line)
	case fn != "":
		return fn
	case line > 0:
		return fmt.Sprintf("line %d", line)
	default:
		return ""
	}
}

// scheduleDebugInfoRefresh coalesces -thread-info / -stack-list-frames on stop.
func (a *DebuggerApp) scheduleDebugInfoRefresh() {
	a.debugInfoMu.Lock()
	if a.debugInfoRunning {
		a.debugInfoPending = true
		a.debugInfoMu.Unlock()
		return
	}
	a.debugInfoRunning = true
	a.debugInfoMu.Unlock()

	go a.runDebugInfoRefresh()
}

func (a *DebuggerApp) runDebugInfoRefresh() {
	for {
		a.refreshThreadsAndStack()
		if scr := a.Screen(); scr != nil {
			_ = scr.PostEvent(tcell.NewEventInterrupt(debugInfoUIMsg{}))
		}

		a.debugInfoMu.Lock()
		if a.debugInfoPending {
			a.debugInfoPending = false
			a.debugInfoMu.Unlock()
			continue
		}
		a.debugInfoRunning = false
		a.debugInfoMu.Unlock()
		return
	}
}

func (a *DebuggerApp) refreshThreadsAndStack() {
	if a.gdbMcp == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if a.threadWidget != nil {
		raw, err := a.gdbMcp.Query(ctx, "-thread-info")
		if err != nil {
			if a.ctx.Log != nil {
				a.ctx.Log.Named("threads").Error(err.Error())
			}
		} else {
			a.threadWidget.SetItems(mcp.ParseThreadInfo(raw))
		}
	}

	if a.callstackWidget != nil {
		raw, err := a.gdbMcp.Query(ctx, "-stack-list-frames")
		if err != nil {
			if a.ctx.Log != nil {
				a.ctx.Log.Named("callstack").Error(err.Error())
			}
		} else {
			a.callstackWidget.SetItems(mcp.ParseStackListFrames(raw))
		}
	}
}

// onBreakpointsChanged publishes a bus event; the Subscribe handler coalesces
// -break-list work. Sources: GDB console, BreakpointWidget, MCP (=breakpoint-*).
func (a *DebuggerApp) onBreakpointsChanged() {
	if a.ctx.Bus == nil {
		return
	}
	platform.Publish(a.ctx.Bus, BreakpointsChangedMsg{})
}

// onBreakpointsChangedMsg is the EventBus Subscribe handler for
// BreakpointsChangedMsg. It coalesces bursts into one in-flight -break-list
// plus at most one trailing refresh (no timer).
func (a *DebuggerApp) onBreakpointsChangedMsg(_ BreakpointsChangedMsg) {
	a.bpRefreshMu.Lock()
	if a.bpRefreshRunning {
		a.bpRefreshPending = true
		a.bpRefreshMu.Unlock()
		return
	}
	a.bpRefreshRunning = true
	a.bpRefreshMu.Unlock()

	go a.runBreakpointRefresh()
}

func (a *DebuggerApp) runBreakpointRefresh() {
	for {
		a.refreshBreakpoints()
		if scr := a.Screen(); scr != nil {
			_ = scr.PostEvent(tcell.NewEventInterrupt(breakpointsUIMsg{}))
		}

		a.bpRefreshMu.Lock()
		if a.bpRefreshPending {
			a.bpRefreshPending = false
			a.bpRefreshMu.Unlock()
			continue
		}
		a.bpRefreshRunning = false
		a.bpRefreshMu.Unlock()
		return
	}
}

// applyCodeStop refreshes the source view for a stop without stealing focus from
// another pane. If the focused pane is already a CodeWidget, switch that pane
// to the stop file; otherwise update the code-slot leaf (Logo or Code) in place.
func (a *DebuggerApp) applyCodeStop(w *widgets.CodeWidget) {
	a.placeCodeInSlot(w)
}

// placeCodeInSlot puts w into the code leaf, replacing LogoWidget or an older CodeWidget.
func (a *DebuggerApp) placeCodeInSlot(w *widgets.CodeWidget) {
	if w == nil || a.tab == nil {
		return
	}
	a.primaryCode = w
	if cw := a.focusedCode(); cw != nil {
		if cw != w {
			_ = a.tab.ReplaceFocusedWidget(w)
		}
		a.rememberCodeLeafFromFocus()
		return
	}
	if _, ok := a.focusedWidget().(*widgets.LogoWidget); ok {
		_ = a.tab.ReplaceFocusedWidget(w)
		a.rememberCodeLeafFromFocus()
		return
	}
	if a.tab.ReplaceMatchingLeafWidget(w, isCodeSlot) {
		a.tab.SetLeafMark(leafMarkCode, a.tab.FindLeaf(isCodeSlot))
	}
}

// ensureSourceFiles re-queries GDB -file-list-exec-source-files and replaces
// AppState.SourceFiles when the parse is non-empty. Always refreshes (does not
// stick to the first cached hit — an early/partial capture used to leave only
// the current frame's main.cpp in :edit / Tab completions).
func (a *DebuggerApp) ensureSourceFiles() {
	if a.gdbMcp == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := a.gdbMcp.Query(ctx, "-file-list-exec-source-files")
	if err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("code").Error(err.Error())
		}
		return
	}
	files := mcp.ParseSourceFileList(raw)
	if len(files) == 0 {
		return
	}
	a.State().SetSourceFiles(files)
	if a.fileListWidget != nil {
		a.fileListWidget.SetItems(files)
	}
}

// refreshBreakpoints queries GDB -break-list and merges into the Breakpoint
// widget's internal list (disabled rows are preserved), then paints CodeWidgets.
func (a *DebuggerApp) refreshBreakpoints() {
	items, ok := a.fetchBreakInfos()
	if !ok {
		// Incomplete/failed capture — keep existing red marks and BP rows.
		return
	}
	a.applyBreakInfos(items)
}

func (a *DebuggerApp) fetchBreakInfos() ([]mcp.BreakInfo, bool) {
	if a.gdbMcp == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := a.gdbMcp.Query(ctx, "-break-list")
	if err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("code").Error(err.Error())
		}
		return nil, false
	}
	// Require a real break-list reply. A stale "(gdb)" from "n" used to yield an
	// empty parse and wipe CodeWidget red marks.
	if !strings.Contains(raw, "BreakpointTable") && !strings.Contains(raw, "bkpt={") {
		return nil, false
	}
	return mcp.ParseBreakList(raw), true
}

func (a *DebuggerApp) applyBreakInfos(gdbItems []mcp.BreakInfo) {
	if a.bpWidget != nil {
		a.bpWidget.MergeFromGDB(gdbItems)
		return // MergeFromGDB calls OnChange → paintCodeBreakmarks
	}
	a.paintCodeBreakmarks(gdbItems)
}

// paintCodeBreakmarks paints line-number gutters from AppState break colors
// (enabled / disabled backgrounds).
func (a *DebuggerApp) paintCodeBreakmarks(items []mcp.BreakInfo) {
	seen := make(map[*widgets.CodeWidget]bool)
	for path, w := range a.fileBuffers {
		if w == nil {
			continue
		}
		w.SetBreakInfos(breaksForFile(items, path))
		seen[w] = true
	}
	if a.primaryCode != nil && !seen[a.primaryCode] {
		if p := a.primaryCode.Path(); p != "" {
			a.primaryCode.SetBreakInfos(breaksForFile(items, p))
		}
	}
}

func (a *DebuggerApp) rebuildCodeBreakGutters() {
	seen := make(map[*widgets.CodeWidget]bool)
	for _, w := range a.fileBuffers {
		if w == nil {
			continue
		}
		w.RebuildBuffer()
		seen[w] = true
	}
	if a.primaryCode != nil && !seen[a.primaryCode] {
		a.primaryCode.RebuildBuffer()
	}
}

// paintCodeWidgetBreaks applies the current BP list (enabled + disabled) to one CodeWidget.
func (a *DebuggerApp) paintCodeWidgetBreaks(w *widgets.CodeWidget, path string) {
	if w == nil {
		return
	}
	var items []mcp.BreakInfo
	if a.bpWidget != nil {
		items = a.bpWidget.Items()
	}
	w.SetBreakInfos(breaksForFile(items, path))
}

func (a *DebuggerApp) onBreakpointListChanged() {
	if a.bpWidget == nil {
		return
	}
	a.paintCodeBreakmarks(a.bpWidget.Items())
	if scr := a.Screen(); scr != nil {
		_ = scr.PostEvent(tcell.NewEventInterrupt(breakpointsUIMsg{}))
	}
}

func breaksForFile(items []mcp.BreakInfo, path string) []mcp.BreakInfo {
	base := filepath.Base(path)
	out := make([]mcp.BreakInfo, 0)
	for _, it := range items {
		if it.File == path || filepath.Base(it.File) == base {
			out = append(out, it)
		}
	}
	return out
}
