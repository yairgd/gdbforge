package models

// BreakInfo is one breakpoint row (GDB -break-list or Delve breakpoints).
type BreakInfo struct {
	Number  int
	Enabled bool
	File    string // fullname preferred, else file / pending path
	Line    int    // 1-based; 0 for address-only breakpoints
	Addr    string // normalized hex address when known (e.g. "0x401126")
}

// BreakMark is an enabled breakpoint location for CodeWidget red marks.
type BreakMark struct {
	File string
	Line int
}

// ThreadInfo is one row from -thread-info / Delve threads.
type ThreadInfo struct {
	ID      string
	State   string
	Name    string
	File    string
	Line    int
	Func    string
	Current bool
}

// StackFrame is one row from -stack-list-frames / Delve stack.
type StackFrame struct {
	Level int
	Func  string
	File  string
	Line  int
	Addr  string
}

// EnabledBreakMarks returns file:line for enabled breakpoints only.
func EnabledBreakMarks(items []BreakInfo) []BreakMark {
	var out []BreakMark
	for _, it := range items {
		if !it.Enabled {
			continue
		}
		out = append(out, BreakMark{File: it.File, Line: it.Line})
	}
	return out
}
