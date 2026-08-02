// Package domain is the shared debugger control surface for peer controllers
// (GUI helpers, MCP/AI tools, future Lua). It sits beside models/ and widgets/
// under gdbforge — not under mcp or gdb.
//
// Implementations live in cmd/gdbforge (DebuggerApp). Transport stays in
// internal/gdb; LLM HTTP stays in internal/mcp.
package domain

// Breakpoint is a domain snapshot row (same facts as the Breakpoints pane).
type Breakpoint struct {
	Number    int    `json:"number"`
	Enabled   bool   `json:"enabled"`
	Condition string `json:"condition,omitempty"`
	File      string `json:"file"`
	Line      int    `json:"line"`
}

// Thread is a domain snapshot row (same facts as the Threads pane).
type Thread struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Name    string `json:"name"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Func    string `json:"func"`
	Current bool   `json:"current"`
}

// Frame is a domain snapshot row (same facts as the Call Stack pane).
type Frame struct {
	Level int    `json:"level"`
	Func  string `json:"func"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Addr  string `json:"addr"`
}

// DebugDomain is implemented by DebuggerApp and consumed by MCP/AI (and later Lua).
// Reads come from shared models; writes must use the same helpers as the GUI and
// mux any GDB Send through Session.WithWrite.
type DebugDomain interface {
	ListBreakpoints() []Breakpoint
	ListThreads() []Thread
	ListFrames() []Frame
	SetBreakpoint(file string, line int) error
	ClearBreakpoint(file string, line int) error
}
