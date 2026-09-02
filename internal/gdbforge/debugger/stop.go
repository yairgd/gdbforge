package debugger

import (
	"github.com/yairgd/gdbforge/internal/gdb"
)

// SourceLocation is a file:line stop or browse point (backend-neutral).
type SourceLocation struct {
	File string
	Line int
	Func string
}

// StopInfo is a backend-neutral *stopped snapshot for the UI stop pipeline.
type StopInfo struct {
	Reason   string
	ThreadID string
	SourceLocation
}

// StopFromGDB maps an MI *stopped record into StopInfo.
func StopFromGDB(stop *gdb.MiStopMsg) *StopInfo {
	if stop == nil {
		return nil
	}
	return &StopInfo{
		Reason:   stop.Reason,
		ThreadID: stop.ThreadId,
		SourceLocation: SourceLocation{
			File: stop.File,
			Line: stop.Line,
			Func: stop.Func,
		},
	}
}

// NeedsUIRefresh reports whether this stop should refresh Code / threads / stack
// (not inferior-exit style reasons). Same rules as gdb.StopNeedsUIRefresh.
func (s *StopInfo) NeedsUIRefresh() bool {
	if s == nil {
		return false
	}
	switch s.Reason {
	case "exited-normally", "exited", "exited-signalled":
		return false
	default:
		return true
	}
}
