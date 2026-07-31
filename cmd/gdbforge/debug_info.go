package main

import (
	"context"
	"fmt"
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
)

// debugInfoHost is the narrow surface debugInfoCtl needs from the composition
// root. DebuggerApp implements it; debugInfoCtl must not depend on *DebuggerApp.
type debugInfoHost interface {
	Backend() backend.Backend
	Session() core.Session
	State() *platform.AppState
	Debug() *debugstate.State
	GdbMcp() *mcp.GdbMcpService
	GDBWidget() *widgets.GDBWidget
	Screen() tcell.Screen
	RequestFrame()
	BumpCodeNav()
	SuppressDlvStopUI()
	isDLV() bool
	showFrameSource(fr models.StackFrame)
	ShowCodeAt(file string, line int) *widgets.CodeWidget
	LogError(area, msg string)
}

// debugInfoCtl owns the Threads / Call Stack domain: shared models, their
// views, the coalesced background refresh, and row activation.
// DebuggerApp wires it; the ctl owns the domain.
type debugInfoCtl struct {
	host     debugInfoHost
	threads  *models.ThreadList
	stack    *models.CallStack
	threadW  *widgets.ThreadWidget
	stackW   *widgets.CallStackWidget
	coalesce coalesceRunner
}

// Threads returns the shared ThreadList (may be nil before InitB).
func (c *debugInfoCtl) Threads() *models.ThreadList { return c.threads }

// Stack returns the shared CallStack (may be nil before InitB).
func (c *debugInfoCtl) Stack() *models.CallStack { return c.stack }

// ThreadWidget returns the Threads view.
func (c *debugInfoCtl) ThreadWidget() *widgets.ThreadWidget { return c.threadW }

// CallStackWidget returns the Call Stack view.
func (c *debugInfoCtl) CallStackWidget() *widgets.CallStackWidget { return c.stackW }

// syncThreadViews pushes the shared ThreadList to the Threads view.
func (c *debugInfoCtl) syncThreadViews() {
	if c.threads == nil || c.threadW == nil {
		return
	}
	c.threadW.SetItems(c.threads.Items())
}

// syncCallStackViews pushes the shared CallStack to the Call Stack view.
func (c *debugInfoCtl) syncCallStackViews() {
	if c.stack == nil || c.stackW == nil {
		return
	}
	c.stackW.SetItems(c.stack.Items())
}

func (c *debugInfoCtl) applyThreadInfos(items []models.ThreadInfo) {
	c.setThreadInfos(items)
	c.syncThreadViews()
}

func (c *debugInfoCtl) applyStackFrames(frames []models.StackFrame) {
	c.setStackFrames(frames)
	c.syncCallStackViews()
}

func (c *debugInfoCtl) setThreadInfos(items []models.ThreadInfo) {
	if c.threads == nil {
		c.threads = &models.ThreadList{}
	}
	c.threads.Set(items)
}

func (c *debugInfoCtl) setStackFrames(frames []models.StackFrame) {
	if c.stack == nil {
		c.stack = &models.CallStack{}
	}
	c.stack.Set(frames)
}

// clearModels empties Threads / Call Stack models and their views
// (inferior exit / kill).
func (c *debugInfoCtl) clearModels() {
	c.setThreadInfos(nil)
	c.setStackFrames(nil)
	c.syncThreadViews()
	c.syncCallStackViews()
}

// selectLevel highlights a stack level in the Call Stack view.
func (c *debugInfoCtl) selectLevel(level int) {
	if c.stackW == nil {
		return
	}
	c.stackW.SelectLevel(level)
}

// selectedLevel returns the highlighted Call Stack level (0 when none).
func (c *debugInfoCtl) selectedLevel() int {
	if c == nil || c.stackW == nil {
		return 0
	}
	if fr, ok := c.stackW.SelectedFrame(); ok {
		return fr.Level
	}
	return 0
}

// scheduleRefresh coalesces -thread-info / -stack-list-frames on stop.
func (c *debugInfoCtl) scheduleRefresh() {
	c.coalesce.Schedule(c.runRefresh)
}

func (c *debugInfoCtl) runRefresh() {
	// Retries: right after *stopped the first -thread-info capture can still
	// be empty/stale; a click later works because GDB is idle. Retry briefly
	// so the Threads pane updates without needing a mouse event.
	var threadsOK, stackOK bool
	for attempt := 0; attempt < 6; attempt++ {
		if attempt > 0 {
			time.Sleep(40 * time.Millisecond)
		}
		threadsOK, stackOK = c.refreshThreadsAndStack()
		if threadsOK && stackOK {
			break
		}
	}
	if h := c.host; h != nil {
		if scr := h.Screen(); scr != nil {
			_ = scr.PostEvent(tcell.NewEventInterrupt(debugInfoUIMsg{}))
		}
	}
}

// refreshThreadsAndStack queries the debugger and updates shared models only.
// Views are synced on the UI thread via debugInfoUIMsg (or sync*Views callers).
// Returns whether each query produced a usable payload.
func (c *debugInfoCtl) refreshThreadsAndStack() (threadsOK, stackOK bool) {
	h := c.host
	if h == nil || h.GdbMcp() == nil || h.Backend() == nil {
		return false, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	logFn := backend.LogFn(func(area, msg string) { h.LogError(area, msg) })
	threads, frames, threadsOK, stackOK := h.Backend().RefreshThreadsAndStack(ctx, h.GdbMcp(), logFn)
	if threadsOK {
		c.setThreadInfos(threads)
		if len(threads) == 0 {
			c.setStackFrames(nil)
		}
	}
	if stackOK {
		c.setStackFrames(frames)
	}
	return threadsOK, stackOK
}

// activateCallStack selects a stack frame in GDB/Delve and shows its source.
// Uses MI for GDB so the console does not print CLI frame listings.
func (c *debugInfoCtl) activateCallStack(fr models.StackFrame) {
	h := c.host
	if h == nil {
		return
	}
	// User is browsing — cancel any in-flight stop refresh that would snap
	// Code back to frame 0.
	h.BumpCodeNav()

	// Drive Code from the selected row first — do not wait on the debugger PTY
	// (Delve `stack` / `goroutines` queries hold the write lock for a long time).
	h.showFrameSource(fr)
	h.RequestFrame()

	if h.GDBWidget() == nil {
		return
	}
	sess := h.Session()
	if sess == nil {
		return
	}
	cmd := fmt.Sprintf("-stack-select-frame %d", fr.Level)
	if h.Backend() != nil {
		cmd = h.Backend().SelectFrameCmd(fr.Level)
	}
	if h.isDLV() {
		// Selecting a call-stack row must update Code from the row's file:line.
		// Sending `frame N` makes Delve re-emit "> …" and dump source, which we
		// used to treat as a new stop (goroutines/stack refresh → snap to frame 0).
		h.SuppressDlvStopUI()
		go gdb.SendCmd(sess, h.State(), h.Debug(), cmd)
		return
	}
	// GDB MI frame select is cheap; keep it on the UI path like before.
	gdb.SendCmd(sess, h.State(), h.Debug(), cmd)
}

// activateThread switches GDB to the selected thread, refreshes stack/threads,
// and shows the current frame source.
// Uses MI for GDB so the console does not print "[Switching to thread …]".
func (c *debugInfoCtl) activateThread(th models.ThreadInfo) {
	h := c.host
	if h == nil || h.GDBWidget() == nil || th.ID == "" {
		return
	}
	sess := h.Session()
	if sess == nil {
		return
	}
	cmd := "-thread-select " + th.ID
	if h.Backend() != nil {
		cmd = h.Backend().SelectThreadCmd(th.ID)
	}
	gdb.SendCmd(sess, h.State(), h.Debug(), cmd)
	c.refreshThreadsAndStack()
	c.syncThreadViews()
	c.syncCallStackViews()

	file, line := th.File, th.Line
	if c.stack != nil {
		if frames := c.stack.Items(); len(frames) > 0 {
			if frames[0].File != "" {
				file, line = frames[0].File, frames[0].Line
			}
		}
	}
	if file != "" {
		w := h.ShowCodeAt(file, line)
		if w != nil && w.Unavailable() {
			fn := th.Func
			if c.stack != nil {
				if frames := c.stack.Items(); len(frames) > 0 && frames[0].Func != "" {
					fn = frames[0].Func
				}
			}
			w.ShowUnavailable(file, formatUnavailableExtra(fn, line))
		}
	}
	h.RequestFrame()
}

// --- Host adapters (ThreadHost / CallStackHost need *DebuggerApp methods) ---

func (a *DebuggerApp) ActivateThread(th models.ThreadInfo)    { a.debugInfo.activateThread(th) }
func (a *DebuggerApp) ActivateCallStack(fr models.StackFrame) { a.debugInfo.activateCallStack(fr) }

// syncFileListViews pushes AppState source files to the FileList view.
func (a *DebuggerApp) syncFileListViews() {
	if a.fileListWidget == nil {
		return
	}
	if files := a.Debug().SourceFiles(); len(files) > 0 {
		a.fileListWidget.SetItems(files)
	}
}

// clearDebugInfoPanes resets Threads, Call Stack, Code pane, and breakpoints
// after the inferior exits (kill / exit).
func (a *DebuggerApp) clearDebugInfoPanes() {
	a.debugInfo.clearModels()
	a.clearCodePane()
	a.clearBreakpointViews()
}

// clearBreakpointViews empties the shared BP model and gutters (UI only).
// Does not clear breakCtl.snapshot — that is saved on quit after kill/exit reset.
func (a *DebuggerApp) clearBreakpointViews() {
	a.breaks.clearModel()
}

// clearCodePane empties Code widgets and restores the logo splash in the code leaf.
func (a *DebuggerApp) clearCodePane() {
	a.bufs.clearAll()
	if a.State() != nil {
		a.Debug().SetCurrentLocation("", 0)
		a.Debug().ClearStopLocation()
	}
	a.placeLogoInCodeSlot()
}

// placeLogoInCodeSlot puts the startup logo back in the code leaf (after kill).
func (a *DebuggerApp) placeLogoInCodeSlot() {
	if a.tab == nil {
		return
	}
	logo := a.logoWidget
	if logo == nil {
		logo = widgets.NewLogoWidget()
		a.logoWidget = logo
	}
	if _, ok := a.focusedWidget().(*widgets.CodeWidget); ok && !a.isGdbLeaf(a.focusedLeaf()) {
		_ = a.tab.ReplaceFocusedWidget(logo)
		a.tab.SetLeafMark(leafMarkCode, a.tab.FindLeaf(isCodeSlot))
		return
	}
	if a.tab.ReplaceMatchingLeafWidget(logo, isCodeSlot) {
		a.tab.SetLeafMark(leafMarkCode, a.tab.FindLeaf(isCodeSlot))
	}
}
