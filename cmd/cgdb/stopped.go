package main

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/gdb"
	"github.com/yairgd/cgdb-go/internal/mcp"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
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
		return
	}
	go func() {
		a.ensureSourceFiles()
		// Do not -break-list on every stop: that flooded the PTY and froze input.
		// Breakpoint list / red marks sync via =breakpoint-* notifies only.
		w := a.ensureCodeBuffer(file)
		if w != nil {
			_ = w.ShowLocation(file, line)
			// New or replaced buffers start with empty gutters — re-apply known marks.
			a.paintCodeWidgetBreaks(w, file)
		}
		if scr := a.Screen(); scr != nil {
			_ = scr.PostEvent(tcell.NewEventInterrupt(codeRefreshMsg{widget: w}))
		}
	}()
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
	platform.Publish(a.ctx.Bus, termui.BreakpointsChangedMsg{})
}

// onBreakpointsChangedMsg is the EventBus Subscribe handler for
// BreakpointsChangedMsg. It coalesces bursts into one in-flight -break-list
// plus at most one trailing refresh (no timer).
func (a *DebuggerApp) onBreakpointsChangedMsg(_ termui.BreakpointsChangedMsg) {
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
// the GDB console. If the focused pane is already a CodeWidget, switch that pane
// to the stop file; otherwise update another visible CodeWidget leaf in place.
func (a *DebuggerApp) applyCodeStop(w *widgets.CodeWidget) {
	if w == nil || a.tab == nil {
		return
	}
	focused := a.tab.FocusedWidget()
	if cw, ok := focused.(*widgets.CodeWidget); ok {
		if cw != w {
			_ = a.tab.ReplaceFocusedWidget(w)
		}
		return
	}
	_ = a.tab.ReplaceMatchingLeafWidget(w, func(x termui.Widget) bool {
		_, ok := x.(*widgets.CodeWidget)
		return ok
	})
}

func (a *DebuggerApp) ensureSourceFiles() {
	if len(a.State().SourceFiles()) > 0 || a.gdbMcp == nil {
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
	if len(files) > 0 {
		a.State().SetSourceFiles(files)
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

// paintCodeBreakmarks paints red line numbers from enabled breakpoints only.
func (a *DebuggerApp) paintCodeBreakmarks(items []mcp.BreakInfo) {
	var enabled []mcp.BreakInfo
	for _, it := range items {
		if it.Enabled {
			enabled = append(enabled, it)
		}
	}
	seen := make(map[*widgets.CodeWidget]bool)
	for path, w := range a.fileBuffers {
		if w == nil {
			continue
		}
		w.SetBreakInfos(breaksForFile(enabled, path))
		seen[w] = true
	}
	if a.primaryCode != nil && !seen[a.primaryCode] {
		if p := a.primaryCode.Path(); p != "" {
			a.primaryCode.SetBreakInfos(breaksForFile(enabled, p))
		}
	}
}

// paintCodeWidgetBreaks applies the current enabled BP list to one CodeWidget.
func (a *DebuggerApp) paintCodeWidgetBreaks(w *widgets.CodeWidget, path string) {
	if w == nil {
		return
	}
	var enabled []mcp.BreakInfo
	if a.bpWidget != nil {
		enabled = a.bpWidget.EnabledBreakInfos()
	}
	w.SetBreakInfos(breaksForFile(enabled, path))
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
