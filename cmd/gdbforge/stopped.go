package main

import (
	"context"
	"fmt"
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
)

type codeRefreshMsg struct {
	widget *widgets.CodeWidget
	// fromStop is set for *stopped / Delve "> " refreshes. Frame/up/down sync
	// must not set this — otherwise we keep snapping Code back to frame 0.
	fromStop bool
	// stopGen is dlv.codeNavGen at stop time; if the user browsed the call
	// stack since then, we skip clobbering Code.
	stopGen uint64
	// stop is applied on the UI thread (not in a background goroutine) so a
	// call-stack browse cannot be overwritten by a racing showCodeAt.
	stop *gdb.MiStopMsg
	// frame is set for console frame/f/up/down. showFrameSource runs on the
	// UI thread with this frame (never present off-thread then re-present
	// with a nil frame — that snapped Asm back to $pc and skipped Code).
	frame *models.StackFrame
}

type breakpointsUIMsg struct{}

type debugInfoUIMsg struct {
	stackOnly bool // kgdb: Call Stack only — skip asm/code side effects
}

func (a *DebuggerApp) maybeClearOutput() {
	if a.outputWidget == nil || !a.Debug().ClearOutput() {
		return
	}
	a.outputWidget.Clear()
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

	// Break/clear while running: SendCmd Ctrl-C + continue. Skip stop UI so the
	// Code blue browse cursor is not yanked to ━━▶ for that transient halt.
	if a.Debug().ConsumeStopUISuppress() {
		return
	}

	// Delve re-prints "> …" (often without [Breakpoint N]) on every `frame N` /
	// call-stack select. That is not a new halt — never run stop UI unless the
	// inferior was actually running (continue/next/step/…).
	if a.isDLV() {
		if a.dlv.consumeSuppressStopUI() {
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

	if a.Debug().KgdbMode() {
		if stop.Func != "" || stop.File != "" {
			a.debugInfo.setStackFrames([]models.StackFrame{{
				Level: 0,
				Func:  stop.Func,
				File:  stop.File,
				Line:  stop.Line,
			}})
		} else {
			a.debugInfo.setStackFrames(nil)
		}
		a.debugInfo.syncCallStackViews()
	}

	// Defer stack refresh until (gdb). kgdb: one -stack-list-frames only.
	if a.Debug().KgdbMode() {
		a.dlv.armStackRefresh()
	} else {
		a.dlv.armDebugInfoRefresh()
	}

	stopGen := a.dlv.codeNavGen
	stopCopy := *stop
	kgdb := a.Debug().KgdbMode()
	go func() {
		if !kgdb {
			a.ensureSourceFiles()
			// Re-query GDB/Delve breakpoints before painting Code. After file /
			// target remote (e.g. :lua remotegdb) the shared model can be stale or
			// path-mismatched; gutters must follow live -break-list / breakpoints.
			a.breaks.refreshAfterStop()
		}
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
	kgdb := a.Debug() != nil && a.Debug().KgdbMode()
	var w *widgets.CodeWidget
	if stop != nil && stop.File != "" {
		w = a.bufs.showCodeAt(stop.File, stop.Line)
		if w != nil && w.Unavailable() {
			w.ShowUnavailable(stop.File, formatUnavailableExtra(stop.Func, stop.Line))
		}
		// Pass stop frame so Asm refreshes to $pc with the new func (not a
		// stale FuncName from the previous browse, e.g. write → main).
		fr := models.StackFrame{Level: 0, Func: stop.Func, File: stop.File, Line: stop.Line}
		a.presentLocation(w, &fr)
	} else if kgdb {
		if stop != nil && stop.Func != "" {
			w = a.bufs.showCodeUnavailable(stop.Func, formatUnavailableExtra("", stop.Line))
			fr := models.StackFrame{Level: 0, Func: stop.Func, Line: stop.Line}
			a.presentLocation(w, &fr)
		}
	} else {
		// No fullname on *stopped — query stack (same path as frame sync).
		a.syncCurrentFrameFromGDB()
		if w = a.activeCodeWidget(); w != nil {
			// showFrameSource already presented.
		} else if stop != nil && stop.Func != "" {
			w = a.bufs.showCodeUnavailable(stop.Func, formatUnavailableExtra("", stop.Line))
			fr := models.StackFrame{Level: 0, Func: stop.Func, Line: stop.Line}
			a.presentLocation(w, &fr)
		}
	}
	return w
}

// onGdbFrameSelected presents Code/Asm from =thread-selected (CLI frame/f/up/down).
// Called on the UI thread with the frame GDB already reported — no MI Query.
func (a *DebuggerApp) onGdbFrameSelected(fr gdb.MiFrameMsg) {
	a.debugInfo.selectLevel(fr.Level)
	a.showFrameSource(models.StackFrame{
		Level: fr.Level,
		Func:  fr.Func,
		File:  fr.File,
		Line:  fr.Line,
		Addr:  fr.Addr,
	})
	a.RequestFrame()
	if a.Debug() != nil && a.Debug().KgdbMode() {
		return
	}
	// Refresh call-stack list off-thread; re-present with the queried frame so
	// Asm browse address stays aligned if the async record omitted addr.
	go func() {
		got, ok := a.fetchCurrentFrameFromGDB()
		if !ok {
			return
		}
		if scr := a.Screen(); scr != nil {
			frCopy := got
			_ = scr.PostEvent(tcell.NewEventInterrupt(codeRefreshMsg{
				frame: &frCopy,
			}))
		}
	}()
}

// onGdbFrameSync refreshes Code / Call Stack after a GDB console frame/f/up/down
// (those do not emit *stopped). Query off-thread; present on the UI thread.
// Used when =thread-selected did not carry a frame (e.g. -stack-select-frame).
func (a *DebuggerApp) onGdbFrameSync() {
	go func() {
		fr, ok := a.fetchCurrentFrameFromGDB()
		if !ok {
			return
		}
		if scr := a.Screen(); scr != nil {
			frCopy := fr
			_ = scr.PostEvent(tcell.NewEventInterrupt(codeRefreshMsg{
				frame: &frCopy,
			}))
		}
	}()
}

// fetchCurrentFrameFromGDB queries the selected frame and updates the call-stack
// model. Does not touch Code/Asm or Call Stack views — UI thread must present.
func (a *DebuggerApp) fetchCurrentFrameFromGDB() (models.StackFrame, bool) {
	if a.gdbMcp == nil {
		return models.StackFrame{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if a.isDLV() {
		raw, err := a.gdbMcp.Query(ctx, "stack")
		if err != nil {
			a.LogError("frame", err.Error())
			return models.StackFrame{}, false
		}
		frames := dlv.ParseStack(raw)
		if len(frames) == 0 {
			return models.StackFrame{}, false
		}
		a.debugInfo.setStackFrames(frames)
		// Delve's `stack` always lists level 0 first; it does not mark the
		// selected frame. Use the level from the last frame/up/down command
		// (or the call-stack highlight) instead of always taking frames[0].
		level := a.dlv.consumeFrameSyncLevel()
		fr := frames[0]
		if found, ok := frameAtLevel(frames, level); ok {
			fr = found
		}
		return fr, true
	}

	raw, err := a.gdbMcp.Query(ctx, "-stack-info-frame")
	if err != nil {
		a.LogError("frame", err.Error())
		return models.StackFrame{}, false
	}
	fr, ok := parse.ParseStackInfoFrame(raw)
	if !ok {
		return models.StackFrame{}, false
	}

	if a.debugInfo.CallStackWidget() != nil || a.debugInfo.Stack() != nil {
		rawStack, err := a.gdbMcp.Query(ctx, "-stack-list-frames")
		if err == nil {
			a.debugInfo.setStackFrames(parse.ParseStackListFrames(rawStack))
		}
	}
	return fr, true
}

// syncCurrentFrameFromGDB fetches the selected frame and presents it (UI thread).
func (a *DebuggerApp) syncCurrentFrameFromGDB() {
	fr, ok := a.fetchCurrentFrameFromGDB()
	if !ok {
		return
	}
	a.debugInfo.syncCallStackViews()
	a.debugInfo.selectLevel(fr.Level)
	a.showFrameSource(fr)
}

func (a *DebuggerApp) showFrameSource(fr models.StackFrame) {
	var w *widgets.CodeWidget
	switch {
	case fr.File != "":
		file := normalizeCodePath(fr.File)
		a.Debug().SetCurrentLocation(file, fr.Line)
		// Browse only: keep ━━▶ on the real stop PC (same as Assembly).
		w = a.bufs.showCodeBrowse(file, fr.Line)
		if w != nil && w.Unavailable() {
			w.ShowUnavailable(file, formatUnavailableExtra(fr.Func, fr.Line))
		}
	case fr.Func != "":
		w = a.bufs.showCodeUnavailable(fr.Func, formatUnavailableExtra("", fr.Line))
	}
	a.presentLocation(w, &fr)
}

// syncCodeFromCallstack moves Code to the first stack frame that has a source
// file. Used after stop refreshes so Ctrl-C / SIGINT still update ━━▶ when the
// current frame is in a library without sources.
//
// When *stopped already carried a file, updateCodeAfterStop owns Code — do not
// jump back to frame 0 here (that races with call-stack j/k / click browse).
func (a *DebuggerApp) syncCodeFromCallstack() {
	stack := a.debugInfo.Stack()
	if stack == nil {
		return
	}
	if a.State() != nil && a.Debug().StopFile() != "" {
		return
	}
	if fr, ok := stack.FirstWithFile(); ok {
		a.Debug().SetStopLocation(fr.File, fr.Line)
		a.showFrameSource(fr)
		return
	}
	if fr, ok := stack.At(0); ok && fr.Func != "" {
		w := a.bufs.showCodeUnavailable(fr.Func, formatUnavailableExtra("", fr.Line))
		a.presentLocation(w, &fr)
	}
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

// presentLocation chooses Code vs Assembly for the location leaf only.
// Like Call Stack / Breakpoints, other panes are never stolen.
//
//	readable source → Code (unless sticky :b asm)
//	no source + asm supported → Assembly (autoAsm)
//	no source + no asm → unavailable Code banner
func (a *DebuggerApp) presentLocation(codeW *widgets.CodeWidget, fr *models.StackFrame) {
	if a == nil {
		return
	}
	unavailable := sourceUnavailable(codeW)
	kgdb := a.Debug() != nil && a.Debug().KgdbMode()

	refreshAsm := func() {
		if kgdb {
			return
		}
		if fr != nil {
			a.asm.syncToFrame(*fr)
			return
		}
		a.asm.armRefresh(true)
	}

	if a.asm.hasSplit() {
		if codeW != nil && !unavailable {
			a.placeCodeInSlot(codeW)
		}
		if a.asm.shouldShow(codeW) {
			refreshAsm()
		}
		return
	}

	if a.asm.PreferAsm() {
		a.asm.setAutoAsm(false)
		if aw := a.asm.Widget(); aw != nil {
			a.asm.placeInSlot(aw)
		}
		refreshAsm()
		return
	}

	if unavailable && a.asm.supported() && a.asm.Widget() != nil {
		a.asm.setAutoAsm(true)
		a.asm.placeInSlot(a.asm.Widget())
		refreshAsm()
		return
	}

	a.asm.setAutoAsm(false)
	if codeW != nil {
		a.placeCodeInSlot(codeW)
	}
}

// applyCodeStop is the UI-thread re-assert of location leaf content after a
// buffer update (same policy as presentLocation).
func (a *DebuggerApp) applyCodeStop(w *widgets.CodeWidget) {
	a.presentLocation(w, nil)
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

// ensureSourceFiles re-queries GDB -file-list-exec-source-files and replaces
// AppState.SourceFiles when the parse is non-empty. Always refreshes (does not
// stick to the first cached hit — an early/partial capture used to leave only
// the current frame's main.cpp in :edit / Tab completions).
func (a *DebuggerApp) ensureSourceFiles() {
	if a.gdbMcp == nil {
		return
	}
	if a.backend == nil || !a.backend.SupportsSourceFileList() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := a.gdbMcp.Query(ctx, "-file-list-exec-source-files")
	if err != nil {
		a.LogError("code", err.Error())
		return
	}
	files := parse.ParseSourceFileList(raw)
	if len(files) == 0 {
		return
	}
	a.Debug().SetSourceFiles(files)
	a.syncFileListViews()
}
