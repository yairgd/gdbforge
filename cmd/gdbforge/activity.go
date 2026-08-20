package main

// Activity is the foreground session-work mini-machine (orthogonal to Mode
// and Confirm). It answers what Ctrl-C / Ctrl-Z mean right now.
type Activity int

const (
	ActivityIdle Activity = iota
	ActivityInferiorRunning
	ActivityLuaJob
)

// activitySnap is a point-in-time read of Activity inputs.
type activitySnap struct {
	InferiorRunning bool
	LuaJob          bool
}

func (a *DebuggerApp) activitySnapshot() activitySnap {
	var s activitySnap
	if a == nil {
		return s
	}
	if a.Debug() != nil {
		s.InferiorRunning = a.Debug().InferiorRunning()
	}
	s.LuaJob = a.lua.JobBusy()
	return s
}

// forCtrlC: LuaJob wins over inferior (cancel script first).
func (s activitySnap) forCtrlC() Activity {
	if s.LuaJob {
		return ActivityLuaJob
	}
	if s.InferiorRunning {
		return ActivityInferiorRunning
	}
	return ActivityIdle
}

// forCtrlZ: InferiorRunning wins over LuaJob (suspend target first).
func (s activitySnap) forCtrlZ() Activity {
	if s.InferiorRunning {
		return ActivityInferiorRunning
	}
	if s.LuaJob {
		return ActivityLuaJob
	}
	return ActivityIdle
}

// onActivityCtrlC applies the Ctrl-C Activity table, then Confirm if needed.
func (a *DebuggerApp) onActivityCtrlC() {
	if a == nil {
		return
	}
	switch a.activitySnapshot().forCtrlC() {
	case ActivityLuaJob:
		if a.lua.cancelJob() {
			a.noteJobCancelled("Ctrl-C")
		}
		a.requestFrameIfReady()
		return
	default:
		if a.confirming() {
			a.onConfirmInterrupt()
			a.requestFrameIfReady()
			return
		}
		a.console.onGdbConsoleInterrupt()
		a.requestFrameIfReady()
	}
}

// onActivityCtrlZ applies the Ctrl-Z Activity table.
func (a *DebuggerApp) onActivityCtrlZ() {
	if a == nil {
		return
	}
	switch a.activitySnapshot().forCtrlZ() {
	case ActivityLuaJob:
		if a.lua.cancelJob() {
			a.noteJobCancelled("Ctrl-Z")
		}
		a.requestFrameIfReady()
		return
	default:
		// InferiorRunning and Idle both go through console suspend:
		// running → SIGTSTP inferior; idle → suspend gdbforge.
		a.console.onGdbConsoleSuspend()
	}
}

func (a *DebuggerApp) noteJobCancelled(via string) {
	if a == nil || a.outputWidget == nil {
		return
	}
	a.outputWidget.AppendHostLine("cancelled (" + via + ")")
}

func (a *DebuggerApp) requestFrameIfReady() {
	if a != nil && a.TermApp != nil {
		a.RequestFrame()
	}
}
