package debugger

import (
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

// FrameInfo is a selected stack frame from a debugger event (backend-neutral).
type FrameInfo struct {
	Level int
	Func  string
	File  string
	Line  int
	Addr  string
}

// FrameFromGDB maps an MI =thread-selected frame record.
func FrameFromGDB(fr *gdb.MiFrameMsg) *FrameInfo {
	if fr == nil {
		return nil
	}
	return &FrameInfo{
		Level: fr.Level,
		Func:  fr.Func,
		File:  fr.File,
		Line:  fr.Line,
		Addr:  fr.Addr,
	}
}

// StackFrame converts to the shared models row type used by widgets.
func (f FrameInfo) StackFrame() models.StackFrame {
	return models.StackFrame{
		Level: f.Level,
		Func:  f.Func,
		File:  f.File,
		Line:  f.Line,
		Addr:  f.Addr,
	}
}
