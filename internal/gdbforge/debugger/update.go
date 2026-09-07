package debugger

import (
	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
)

// SessionState mirrors gdb.GdbState for running/done/error discrimination.
type SessionState = gdb.GdbState

const (
	StateDone    = gdb.Done
	StateError   = gdb.Error
	StateRunning = gdb.Running
)

// ConsoleUpdate holds backend-neutral fields consumed by the console bridge
// side-effect pipeline (stop, prompt, breakpoints, frame sync).
type ConsoleUpdate struct {
	Stopped            *StopInfo
	InferiorExited     bool
	PromptReady        bool
	State              SessionState
	BreakpointsChanged bool
	FrameSelected      *FrameInfo
}

// FromGDBUpdate maps a GDB MI parse batch into ConsoleUpdate.
func FromGDBUpdate(u gdb.MiUpdate) ConsoleUpdate {
	var fr *FrameInfo
	if u.FrameSelected != nil {
		fr = FrameFromGDB(u.FrameSelected)
	}
	return ConsoleUpdate{
		Stopped:            StopFromGDB(u.Stopped),
		InferiorExited:     u.InferiorExited,
		PromptReady:        u.PromptReady,
		State:              u.State,
		BreakpointsChanged: u.BreakpointsChanged,
		FrameSelected:      fr,
	}
}

// FromDLVUpdate maps a Delve CLI parse batch into ConsoleUpdate.
func FromDLVUpdate(u dlv.Update) ConsoleUpdate {
	return ConsoleUpdate{
		Stopped:            StopFromGDB(u.Stopped),
		InferiorExited:     u.InferiorExited,
		PromptReady:        u.PromptReady,
		State:              u.State,
		BreakpointsChanged: u.BreakpointsChanged,
	}
}
