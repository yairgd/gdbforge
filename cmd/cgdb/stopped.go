package main

import (
	"context"
	"path/filepath"
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

// onGdbStopped updates AppState / CodeWidget when GDB hits a breakpoint or steps.
func (a *DebuggerApp) onGdbStopped(stop *gdb.MiStopMsg) {
	if stop == nil || stop.File == "" {
		return
	}
	file := stop.File
	line := stop.Line

	a.State().SetCurrentLocation(file, line)

	go func() {
		a.ensureSourceFiles()
		// Do not -break-list on every stop: that flooded the PTY and froze input.
		// Breakpoint list / red marks sync via =breakpoint-* notifies only.
		w := a.ensureCodeBuffer(file)
		if w != nil {
			_ = w.ShowLocation(file, line)
		}
		if scr := a.Screen(); scr != nil {
			_ = scr.PostEvent(tcell.NewEventInterrupt(codeRefreshMsg{widget: w}))
		}
	}()
}

// onBreakpointsChanged is the single sync point for breakpoint UI:
// GDB console (PTYOwnerUI), BreakpointWidget (PTYOwnerApp), and MCP
// (PTYOwnerMCP) all end up here via MI =breakpoint-*.
func (a *DebuggerApp) onBreakpointsChanged() {
	a.scheduleBreakpointRefresh()
}

// scheduleBreakpointRefresh coalesces bursts (created + modified + MCP backup)
// into one in-flight -break-list, with at most one trailing refresh.
func (a *DebuggerApp) scheduleBreakpointRefresh() {
	a.bpRefreshMu.Lock()
	if a.bpRefreshRunning {
		a.bpRefreshPending = true
		a.bpRefreshMu.Unlock()
		return
	}
	a.bpRefreshRunning = true
	a.bpRefreshMu.Unlock()

	go func() {
		for {
			time.Sleep(30 * time.Millisecond)
			a.refreshBreakpoints()
			a.publishBreakpointsChanged()
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
	}()
}

func (a *DebuggerApp) publishBreakpointsChanged() {
	if a.ctx.Bus == nil {
		return
	}
	platform.Publish(a.ctx.Bus, termui.BreakpointsChangedMsg{})
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
	items := a.fetchBreakInfos()
	a.applyBreakInfos(items)
}

func (a *DebuggerApp) fetchBreakInfos() []mcp.BreakInfo {
	if a.gdbMcp == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := a.gdbMcp.Query(ctx, "-break-list")
	if err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("code").Error(err.Error())
		}
		return nil
	}
	return mcp.ParseBreakList(raw)
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
	if a.fileBuffers == nil {
		return
	}
	var enabled []mcp.BreakInfo
	for _, it := range items {
		if it.Enabled {
			enabled = append(enabled, it)
		}
	}
	for path, w := range a.fileBuffers {
		w.SetBreakInfos(breaksForFile(enabled, path))
	}
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
	var out []mcp.BreakInfo
	for _, it := range items {
		if it.File == path || filepath.Base(it.File) == base {
			out = append(out, it)
		}
	}
	return out
}
