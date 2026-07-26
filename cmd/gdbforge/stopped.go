package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"path/filepath"
	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
)

type codeRefreshMsg struct {
	widget *widgets.CodeWidget
	// fromStop is set for *stopped / Delve "> " refreshes. Frame/up/down sync
	// must not set this — otherwise we keep snapping Code back to frame 0.
	fromStop bool
	// stopGen is App.codeNavGen at stop time; if the user browsed the call
	// stack since then, we skip clobbering Code.
	stopGen uint64
	// stop is applied on the UI thread (not in a background goroutine) so a
	// call-stack browse cannot be overwritten by a racing showCodeAt.
	stop *gdb.MiStopMsg
}

type breakpointsUIMsg struct{}

type debugInfoUIMsg struct{}

func (a *DebuggerApp) maybeClearOutput() {
	if a.outputWidget == nil || !a.Debug().ClearOutput() {
		return
	}
	a.outputWidget.Clear()
}

// maybeBreakMain inserts a default entry breakpoint when AppState.BreakMain is set.
func (a *DebuggerApp) maybeBreakMain() {
	if a.gdbWidget == nil || !a.Debug().BreakMain() {
		return
	}
	sess := a.GDB()
	if sess == nil {
		return
	}
	cmd := "break main"
	if a.isDLV() {
		cmd = "break main.main"
	}
	gdb.SendCmd(sess, a.State(), a.Debug(), cmd)
}

// onGdbStopped updates AppState / CodeWidget when GDB hits a breakpoint or steps,
// and marks Threads / Call stack for refresh after the next MI prompt.
func (a *DebuggerApp) onGdbStopped(stop *gdb.MiStopMsg) {
	if stop == nil {
		return
	}
	wasRunning := a.State() != nil && a.Debug().InferiorRunning()
	a.Debug().SetInferiorRunning(false)
	needsRefresh := gdb.StopNeedsUIRefresh(stop)
	if a.isDLV() {
		needsRefresh = dlv.StopNeedsUIRefresh(stop)
	}
	if !needsRefresh {
		// exited / kill — clear Threads + Call Stack (do not query stack).
		a.clearDebugInfoPanes()
		return
	}

	// Delve re-prints "> …" (often without [Breakpoint N]) on every `frame N` /
	// call-stack select. That is not a new halt — never run stop UI unless the
	// inferior was actually running (continue/next/step/…).
	if a.isDLV() {
		if a.dlvSuppressStopUI > 0 {
			a.dlvSuppressStopUI--
			return
		}
		if !wasRunning {
			return
		}
	}

	file := stop.File
	line := stop.Line
	if file != "" {
		a.Debug().SetStopLocation(file, line)
		a.Debug().SetCurrentLocation(file, line)
	}

	// Defer -thread-info / -stack-list-frames until (gdb), with a short
	// fallback timer if PromptReady is missed (Threads then only updated on click).
	a.armDebugInfoRefresh()

	stopGen := a.codeNavGen
	stopCopy := *stop
	go func() {
		a.ensureSourceFiles()
		if scr := a.Screen(); scr != nil {
			// Apply Code on the UI thread after gen check (see codeRefreshMsg).
			_ = scr.PostEvent(tcell.NewEventInterrupt(codeRefreshMsg{
				fromStop: true,
				stopGen:  stopGen,
				stop:     &stopCopy,
			}))
		}
	}()
}

// updateCodeAfterStop moves ━━▶ to the stop location. Prefers *stopped frame;
// if that has no source file (common for SIGINT in libc), uses the first
// call-stack frame that has a file after a stack query.
func (a *DebuggerApp) updateCodeAfterStop(stop *gdb.MiStopMsg) *widgets.CodeWidget {
	if stop != nil && stop.File != "" {
		w := a.showCodeAt(stop.File, stop.Line)
		if w != nil && w.Unavailable() {
			w.ShowUnavailable(stop.File, formatUnavailableExtra(stop.Func, stop.Line))
		}
		return w
	}
	// No fullname on *stopped — query stack (same path as frame sync).
	a.syncCurrentFrameFromGDB()
	if w := a.activeCodeWidget(); w != nil {
		return w
	}
	if stop != nil && stop.Func != "" {
		return a.showCodeUnavailable(stop.Func, formatUnavailableExtra("", stop.Line))
	}
	return nil
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
	a.Debug().SetCurrentLocation(file, line)
	if created && !w.Unavailable() {
		a.paintCodeWidgetBreaks(w, file)
	}
	a.placeCodeInSlot(w)
	return w
}

// showCodeBrowse loads file and moves the blue code cursor to line without
// moving ━━▶ (program counter). Used for Breakpoints list navigation.
func (a *DebuggerApp) showCodeBrowse(file string, line int) *widgets.CodeWidget {
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
	_ = w.ShowSelection(file, line)
	a.Debug().SetCurrentLocation(file, line)
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

// onCallStackActivate selects a stack frame in GDB/Delve and shows its source.
// Uses MI for GDB so the console does not print CLI frame listings.
func (a *DebuggerApp) onCallStackActivate(fr models.StackFrame) {
	// User is browsing — cancel any in-flight stop refresh that would snap
	// Code back to frame 0.
	a.codeNavGen++

	// Drive Code from the selected row first — do not wait on the debugger PTY
	// (Delve `stack` / `goroutines` queries hold the write lock for a long time).
	a.showFrameSource(fr)
	a.RequestFrame()

	if a.gdbWidget == nil {
		return
	}
	sess := a.GDB()
	if sess == nil {
		return
	}
	cmd := fmt.Sprintf("-stack-select-frame %d", fr.Level)
	if a.isDLV() {
		// Selecting a call-stack row must update Code from the row's file:line.
		// Sending `frame N` makes Delve re-emit "> …" and dump source, which we
		// used to treat as a new stop (goroutines/stack refresh → snap to frame 0).
		a.dlvSuppressStopUI++
		cmd = fmt.Sprintf("frame %d", fr.Level)
		go gdb.SendCmd(sess, a.State(), a.Debug(), cmd)
		return
	}
	// GDB MI frame select is cheap; keep it on the UI path like before.
	gdb.SendCmd(sess, a.State(), a.Debug(), cmd)
}

// onGdbFrameSync refreshes Code / Call Stack after a GDB console frame/f/up/down
// (those do not emit *stopped).
func (a *DebuggerApp) onGdbFrameSync() {
	go func() {
		a.syncCurrentFrameFromGDB()
		if scr := a.Screen(); scr != nil {
			// Not fromStop: must not SelectLevel(0) / force the stop file.
			_ = scr.PostEvent(tcell.NewEventInterrupt(codeRefreshMsg{
				widget: a.activeCodeWidget(),
			}))
		}
	}()
}

func (a *DebuggerApp) syncCurrentFrameFromGDB() {
	if a.gdbMcp == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if a.isDLV() {
		raw, err := a.gdbMcp.Query(ctx, "stack")
		if err != nil {
			if a.ctx.Log != nil {
				a.ctx.Log.Named("frame").Error(err.Error())
			}
			return
		}
		frames := dlv.ParseStack(raw)
		if len(frames) == 0 {
			return
		}
		a.applyStackFrames(frames)
		// Delve's `stack` always lists level 0 first; it does not mark the
		// selected frame. Use the level from the last frame/up/down command
		// (or the call-stack highlight) instead of always taking frames[0].
		level := a.consumeFrameSyncLevel()
		fr := frames[0]
		if found, ok := frameAtLevel(frames, level); ok {
			fr = found
		}
		if a.callstackWidget != nil {
			a.callstackWidget.SelectLevel(fr.Level)
		}
		a.showFrameSource(fr)
		return
	}

	raw, err := a.gdbMcp.Query(ctx, "-stack-info-frame")
	if err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("frame").Error(err.Error())
		}
		return
	}
	fr, ok := parse.ParseStackInfoFrame(raw)
	if !ok {
		return
	}

	// Keep the callstack list in sync and highlight the selected level.
	if a.callstackWidget != nil || a.callstack != nil {
		rawStack, err := a.gdbMcp.Query(ctx, "-stack-list-frames")
		if err == nil {
			a.applyStackFrames(parse.ParseStackListFrames(rawStack))
		}
		if a.callstackWidget != nil {
			a.callstackWidget.SelectLevel(fr.Level)
		}
	}

	a.showFrameSource(fr)
}

func (a *DebuggerApp) showFrameSource(fr models.StackFrame) {
	var w *widgets.CodeWidget
	switch {
	case fr.File != "":
		file := normalizeCodePath(fr.File)
		a.Debug().SetCurrentLocation(file, fr.Line)
		// Level 0 follows the real PC (━━▶); other frames browse with the blue
		// cursor so the stop mark stays put — same idea as the Breakpoints list.
		if fr.Level == 0 {
			w = a.showCodeAt(file, fr.Line)
		} else {
			w = a.showCodeBrowse(file, fr.Line)
		}
		if w != nil && w.Unavailable() {
			w.ShowUnavailable(file, formatUnavailableExtra(fr.Func, fr.Line))
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

// syncCodeFromCallstack moves Code to the first stack frame that has a source
// file. Used after stop refreshes so Ctrl-C / SIGINT still update ━━▶ when the
// current frame is in a library without sources.
//
// When *stopped already carried a file, updateCodeAfterStop owns Code — do not
// jump back to frame 0 here (that races with call-stack j/k / click browse).
func (a *DebuggerApp) syncCodeFromCallstack() {
	if a.callstack == nil {
		return
	}
	if a.State() != nil && a.Debug().StopFile() != "" {
		return
	}
	if fr, ok := a.callstack.FirstWithFile(); ok {
		a.Debug().SetStopLocation(fr.File, fr.Line)
		a.showFrameSource(fr)
		return
	}
	if fr, ok := a.callstack.At(0); ok && fr.Func != "" {
		w := a.showCodeUnavailable(fr.Func, formatUnavailableExtra("", fr.Line))
		a.applyCodeStop(w)
	}
}

// onBreakpointActivate shows the source at the selected breakpoint location
// with the blue browse cursor — ━━▶ stays on the real program counter.
func (a *DebuggerApp) onBreakpointActivate(bp models.BreakInfo) {
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

// onThreadActivate switches GDB to the selected thread, refreshes stack/threads,
// and shows the current frame source.
// Uses MI for GDB so the console does not print "[Switching to thread …]".
func (a *DebuggerApp) onThreadActivate(th models.ThreadInfo) {
	if a.gdbWidget == nil || th.ID == "" {
		return
	}
	sess := a.GDB()
	if sess == nil {
		return
	}
	cmd := "-thread-select " + th.ID
	if a.isDLV() {
		cmd = "goroutine " + th.ID
	}
	gdb.SendCmd(sess, a.State(), a.Debug(), cmd)
	a.refreshThreadsAndStack()
	a.syncThreadViews()
	a.syncCallStackViews()

	file, line := th.File, th.Line
	if a.callstack != nil {
		if frames := a.callstack.Items(); len(frames) > 0 {
			if frames[0].File != "" {
				file, line = frames[0].File, frames[0].Line
			}
		}
	}
	if file != "" {
		w := a.showCodeAt(file, line)
		if w != nil && w.Unavailable() {
			fn := th.Func
			if a.callstack != nil {
				if frames := a.callstack.Items(); len(frames) > 0 && frames[0].Func != "" {
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

// armDebugInfoRefresh marks a post-stop threads/stack refresh and starts a
// fallback timer so we still refresh if PromptReady is missed.
func (a *DebuggerApp) armDebugInfoRefresh() {
	a.pendingDebugInfo = true
	go func() {
		time.Sleep(120 * time.Millisecond)
		a.triggerPendingDebugInfo()
	}()
}

// triggerPendingDebugInfo runs a scheduled refresh once if still armed.
func (a *DebuggerApp) triggerPendingDebugInfo() {
	if !a.pendingDebugInfo {
		return
	}
	a.pendingDebugInfo = false
	a.scheduleDebugInfoRefresh()
}

func (a *DebuggerApp) runDebugInfoRefresh() {
	for {
		// Retries: right after *stopped the first -thread-info capture can still
		// be empty/stale; a click later works because GDB is idle. Retry briefly
		// so the Threads pane updates without needing a mouse event.
		var threadsOK, stackOK bool
		for attempt := 0; attempt < 6; attempt++ {
			if attempt > 0 {
				time.Sleep(40 * time.Millisecond)
			}
			threadsOK, stackOK = a.refreshThreadsAndStack()
			if threadsOK && stackOK {
				break
			}
		}
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

// refreshThreadsAndStack queries the debugger and updates shared models only.
// Views are synced on the UI thread via debugInfoUIMsg (or sync*Views callers).
// Returns whether each query produced a usable payload.
func (a *DebuggerApp) refreshThreadsAndStack() (threadsOK, stackOK bool) {
	if a.gdbMcp == nil {
		return false, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if a.isDLV() {
		raw, err := a.gdbMcp.Query(ctx, "goroutines")
		if err != nil {
			if a.ctx.Log != nil {
				a.ctx.Log.Named("threads").Error(err.Error())
			}
		} else if strings.Contains(strings.ToLower(raw), "goroutine") {
			items := dlv.ParseGoroutines(raw)
			a.setThreadInfos(items)
			threadsOK = true
			if len(items) == 0 {
				a.setStackFrames(nil)
			}
		}

		raw, err = a.gdbMcp.Query(ctx, "stack")
		if err != nil {
			if a.ctx.Log != nil {
				a.ctx.Log.Named("callstack").Error(err.Error())
			}
		} else {
			frames := dlv.ParseStack(raw)
			if len(frames) > 0 {
				a.setStackFrames(frames)
				stackOK = true
			}
		}
		return threadsOK, stackOK
	}

	raw, err := a.gdbMcp.Query(ctx, "-thread-info")
	if err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("threads").Error(err.Error())
		}
	} else if strings.Contains(raw, "threads=") {
		// Must see threads= (not bare id= — that matches thread-id= on *stopped).
		items := parse.ParseThreadInfo(raw)
		a.setThreadInfos(items)
		threadsOK = true
		// After kill, -thread-info is often empty while -stack-list-frames fails
		// (no stack=) — clear stale frames when there are no threads.
		if len(items) == 0 {
			a.setStackFrames(nil)
		}
	} else if a.ctx.Log != nil {
		a.ctx.Log.Named("threads").Error("incomplete -thread-info capture")
	}

	raw, err = a.gdbMcp.Query(ctx, "-stack-list-frames")
	if err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("callstack").Error(err.Error())
		}
	} else if strings.Contains(raw, "stack=") {
		a.setStackFrames(parse.ParseStackListFrames(raw))
		stackOK = true
	} else if a.ctx.Log != nil {
		a.ctx.Log.Named("callstack").Error("incomplete -stack-list-frames capture")
	}
	return threadsOK, stackOK
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
	if a.isDLV() && a.dlvConfirm.Confirming() {
		a.dlvBPDeferred = true
		return
	}
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
// Never places code onto the fixed GDB layout leaf.
func (a *DebuggerApp) placeCodeInSlot(w *widgets.CodeWidget) {
	if w == nil || a.tab == nil {
		return
	}
	a.primaryCode = w
	if !a.isGdbLeaf(a.focusedLeaf()) {
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
	}
	// Prefer the remembered code leaf so call-stack / BP focus still updates
	// the source pane (ReplaceMatchingLeafWidget only scans non-focused leaves
	// and can miss when the mark points at a specific split).
	if leaf := a.findCodeLeaf(); leaf != nil && !a.isGdbLeaf(leaf) {
		leaf.SetWidget(w)
		a.tab.SetLeafMark(leafMarkCode, leaf)
		return
	}
	if a.tab.ReplaceMatchingLeafWidget(w, isCodeSlot) {
		a.tab.SetLeafMark(leafMarkCode, a.tab.FindLeaf(isCodeSlot))
	}
}

// frameAtLevel returns the stack frame with the given level.
func frameAtLevel(frames []models.StackFrame, level int) (models.StackFrame, bool) {
	for _, fr := range frames {
		if fr.Level == level {
			return fr, true
		}
	}
	return models.StackFrame{}, false
}

// consumeFrameSyncLevel returns the Delve frame level to show after frame/up/down
// (or call-stack activate), then clears the pending flag.
func (a *DebuggerApp) consumeFrameSyncLevel() int {
	if a.pendingFrameLevelSet {
		level := a.pendingFrameLevel
		a.pendingFrameLevelSet = false
		a.pendingFrameLevel = 0
		if level < 0 {
			return 0
		}
		return level
	}
	if a.callstackWidget != nil {
		if fr, ok := a.callstackWidget.SelectedFrame(); ok {
			return fr.Level
		}
	}
	return 0
}

// noteFrameSyncLevel records which stack level a pending Delve frame sync should show.
func (a *DebuggerApp) noteFrameSyncLevel(level int) {
	if level < 0 {
		level = 0
	}
	a.pendingFrameLevel = level
	a.pendingFrameLevelSet = true
}

// ensureSourceFiles re-queries GDB -file-list-exec-source-files and replaces
// AppState.SourceFiles when the parse is non-empty. Always refreshes (does not
// stick to the first cached hit — an early/partial capture used to leave only
// the current frame's main.cpp in :edit / Tab completions).
func (a *DebuggerApp) ensureSourceFiles() {
	if a.gdbMcp == nil {
		return
	}
	if a.isDLV() {
		// Delve has no direct analogue of -file-list-exec-source-files in MVP.
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
	files := parse.ParseSourceFileList(raw)
	if len(files) == 0 {
		return
	}
	a.Debug().SetSourceFiles(files)
	a.syncFileListViews()
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

func (a *DebuggerApp) fetchBreakInfos() ([]models.BreakInfo, bool) {
	if a.gdbMcp == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if a.isDLV() {
		raw, err := a.gdbMcp.Query(ctx, "breakpoints")
		if err != nil {
			if a.ctx.Log != nil {
				a.ctx.Log.Named("code").Error(err.Error())
			}
			return nil, false
		}
		items := dlv.ParseBreakpoints(raw)
		low := strings.ToLower(raw)
		if len(items) == 0 {
			// Real empty list from Delve.
			if strings.Contains(low, "no breakpoints") {
				return items, true
			}
			// Capture often includes only runtime-* named BPs; if we could not
			// parse any numeric user BP, keep the optimistic UI list.
			if strings.Contains(low, "breakpoint") {
				return nil, false
			}
			return nil, false
		}
		return items, true
	}

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
	return parse.ParseBreakList(raw), true
}

// paintCodeBreakmarks paints line-number gutters from AppState break colors
// (enabled / disabled backgrounds).
func (a *DebuggerApp) paintCodeBreakmarks(items []models.BreakInfo) {
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

func breaksForFile(items []models.BreakInfo, path string) []models.BreakInfo {
	base := filepath.Base(path)
	out := make([]models.BreakInfo, 0)
	for _, it := range items {
		if it.File == path || filepath.Base(it.File) == base {
			out = append(out, it)
		}
	}
	return out
}
