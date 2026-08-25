package main

import (
	"time"

	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/platform"
)

// dlvCtl owns Delve confirm-gate state and stop/frame-sync bookkeeping used by
// the console bridge and stop pipeline (not only Delve — GDB frame-nav shares
// pendingFrameSync / codeNavGen).
type dlvCtl struct {
	app *DebuggerApp

	// confirm tracks Delve [Y/n]? prompts (suspended BP after exit, etc.).
	confirm dlv.ConfirmGate
	// bpDeferred is set when a BP refresh was skipped while confirm is active.
	bpDeferred bool

	pendingFrameSync     bool
	pendingFrameLevel    int // Delve: level to show after frame/up/down
	pendingFrameLevelSet bool
	pendingDebugInfo     bool // refresh threads/stack after *stopped once prompt is ready
	pendingStackRefresh  bool // kgdb: -stack-list-frames only after *stopped

	// codeNavGen increments when the user browses away from the stop frame
	// (call stack / frame cmd). Late stop refreshes with an older gen are ignored.
	codeNavGen uint64
	// suppressStopUI counts Delve frame/up/down ops whose re-emitted "> …"
	// lines must not run stop UI (would snap Code back to frame 0).
	suppressStopUI int
}

// noteStackNavGDB marks a GDB stack-nav CLI command for a post-prompt frame sync.
func (c *dlvCtl) noteStackNavGDB() {
	c.codeNavGen++
	c.pendingFrameSync = true
}

// noteStackNavDLV marks a Delve stack-nav command: suppress stop UI and sync
// the selected frame level after the next prompt.
func (c *dlvCtl) noteStackNavDLV(cmd string, curLevel int) {
	c.codeNavGen++
	c.suppressStopUI++
	c.pendingFrameSync = true
	if level, ok := dlv.FrameNavTargetLevel(cmd, curLevel); ok {
		c.noteFrameSyncLevel(level)
	}
}

// consumeFrameSyncLevel returns the Delve frame level to show after frame/up/down
// (or call-stack activate), then clears the pending flag.
func (c *dlvCtl) consumeFrameSyncLevel() int {
	a := c.app
	if c.pendingFrameLevelSet {
		level := c.pendingFrameLevel
		c.pendingFrameLevelSet = false
		c.pendingFrameLevel = 0
		if level < 0 {
			return 0
		}
		return level
	}
	if a != nil {
		return a.debugInfo.selectedLevel()
	}
	return 0
}

// noteFrameSyncLevel records which stack level a pending Delve frame sync should show.
func (c *dlvCtl) noteFrameSyncLevel(level int) {
	if level < 0 {
		level = 0
	}
	c.pendingFrameLevel = level
	c.pendingFrameLevelSet = true
}

// armStackRefresh marks a post-stop call-stack refresh (kgdb: one stack query).
// Runs from TriggerPendingStackRefreshIfReady when (gdb) is ready — not on a
// short timer (kgdb serial prompt can take >120ms; early query left only frame 0).
func (c *dlvCtl) armStackRefresh() {
	c.pendingStackRefresh = true
}

func (c *dlvCtl) triggerPendingStackRefresh() {
	a := c.app
	if a == nil || !c.pendingStackRefresh {
		return
	}
	c.pendingStackRefresh = false
	a.debugInfo.scheduleStackRefresh()
}

// armDebugInfoRefresh marks a post-stop threads/stack refresh and starts a
// fallback timer so we still refresh if PromptReady is missed.
func (c *dlvCtl) armDebugInfoRefresh() {
	c.pendingDebugInfo = true
	go func() {
		time.Sleep(120 * time.Millisecond)
		c.triggerPendingDebugInfo()
	}()
}

// triggerPendingDebugInfo runs a scheduled refresh once if still armed.
func (c *dlvCtl) triggerPendingDebugInfo() {
	a := c.app
	if a == nil || !c.pendingDebugInfo {
		return
	}
	c.pendingDebugInfo = false
	a.debugInfo.scheduleRefresh()
}

// clearPendingOnTeardown drops in-flight sync flags when the Delve session ends.
func (c *dlvCtl) clearPendingOnTeardown() {
	c.pendingDebugInfo = false
	c.pendingStackRefresh = false
	c.pendingFrameSync = false
}

// bumpCodeNav invalidates in-flight stop paints (user browsing call stack / frame).
func (c *dlvCtl) bumpCodeNav() {
	c.codeNavGen++
}

// deferBPRefresh skips -break-list until Delve leaves the [Y/n]? prompt.
func (c *dlvCtl) deferBPRefresh() {
	c.bpDeferred = true
}

// takeDeferredBP returns whether a deferred BP refresh was pending and clears it.
func (c *dlvCtl) takeDeferredBP() bool {
	if !c.bpDeferred {
		return false
	}
	c.bpDeferred = false
	return true
}

// consumeSuppressStopUI decrements a suppress token; returns true if stop UI
// should be skipped for this stop event.
func (c *dlvCtl) consumeSuppressStopUI() bool {
	if c.suppressStopUI <= 0 {
		return false
	}
	c.suppressStopUI--
	return true
}

// clearSuppressStopUI drops unused frame-nav suppress tokens once Delve is idle.
func (c *dlvCtl) clearSuppressStopUI() {
	c.suppressStopUI = 0
}

// applyPendingFrameSync clears pendingFrameSync when ready and reports whether
// onGdbFrameSync should run (prompt ready, not an error state).
func (c *dlvCtl) applyPendingFrameSync(promptReady, isError bool) bool {
	if !c.pendingFrameSync {
		return false
	}
	if isError {
		c.pendingFrameSync = false
		return false
	}
	if promptReady {
		c.pendingFrameSync = false
		return true
	}
	return false
}

func (c *dlvCtl) Register(bus *platform.EventBus) {
	platform.Subscribe(bus, c.onCodeRefresh)
}

func (c *dlvCtl) onCodeRefresh(msg codeRefreshMsg) {
	a := c.app
	if a == nil {
		return
	}
	if msg.fromStop {
		if msg.stopGen != c.codeNavGen {
			a.RequestFrame()
			return
		}
		a.debugInfo.selectLevel(0)
		if msg.stop != nil {
			_ = a.updateCodeAfterStop(msg.stop)
			if a.breaks.List() != nil {
				a.breaks.paintCodeMarks(a.breaks.Items())
			}
			a.RequestFrame()
			return
		}
	}
	if msg.frame != nil {
		a.debugInfo.syncCallStackViews()
		a.debugInfo.selectLevel(msg.frame.Level)
		a.showFrameSource(*msg.frame)
		a.RequestFrame()
		return
	}
	if msg.widget != nil {
		a.presentLocation(msg.widget, nil)
	}
	a.RequestFrame()
}
