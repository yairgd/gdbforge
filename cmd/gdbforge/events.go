package main

// BreakpointsChangedMsg is published when GDB reports a breakpoint change
// (=breakpoint-* / MCP / Space / BreakpointWidget). DebuggerApp Subscribes and
// coalesces a -break-list refresh so CodeWidget marks and the Breakpoint list
// stay in sync. Lives in cmd/gdbforge (not termui) so the Tab shell stays generic.
type BreakpointsChangedMsg struct{}

func (m BreakpointsChangedMsg) Type() string { return "BreakpointsChangedMsg" }
