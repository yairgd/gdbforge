package pydbg

import (
	"strconv"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

// DAP body shapes (only the fields pydbg reads).

type dapSource struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type dapFrame struct {
	ID     int        `json:"id"`
	Name   string     `json:"name"`
	Source *dapSource `json:"source"`
	Line   int        `json:"line"`
}

type dapThread struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (f dapFrame) file() string {
	if f.Source == nil {
		return ""
	}
	return f.Source.Path
}

func stackFrameFromDAP(level int, f dapFrame) models.StackFrame {
	return models.StackFrame{Level: level, Func: f.Name, File: f.file(), Line: f.Line}
}

func threadInfoFromDAP(t dapThread, state string, current bool) models.ThreadInfo {
	return models.ThreadInfo{ID: strconv.Itoa(t.ID), Name: t.Name, State: state, Current: current}
}

func breakInfoFromEntry(b *breakpoint) models.BreakInfo {
	return models.BreakInfo{
		Number: b.number, Enabled: b.enabled, Condition: b.cond,
		File: b.file, Line: b.line, Type: "breakpoint",
	}
}
