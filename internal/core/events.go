package core

// Event is the domain event interface for debugger backends.
// gdbforge's terminal UI bus uses termui.Event instead.
type Event interface {
	Type() string
}

// PtyOutputMsg carries a raw PTY chunk from any ptyx-backed session
// (GDB, exec/shell, …). Used by core.Session.Subscribe for MCP/REST.
type PtyOutputMsg struct {
	Data string
	Err  error
}

func (PtyOutputMsg) Type() string { return "PtyOutputMsg" }

// GdbOutputMsg is a UI-routed GDB PTY chunk (EventInterrupt → GDBWidget).
type GdbOutputMsg struct {
	Data string
	Err  error
}

func (GdbOutputMsg) Type() string { return "GdbOutputMsg" }

// ExecOutputMsg is a UI-routed exec/shell PTY chunk (EventInterrupt → ExecWidget).
type ExecOutputMsg struct {
	Data string
	Err  error
}

func (ExecOutputMsg) Type() string { return "ExecOutputMsg" }

// InferiorOutputMsg is a UI-routed chunk from the debugged program's PTY
// (EventInterrupt → IO / OutputWidget), after -inferior-tty-set.
type InferiorOutputMsg struct {
	Data string
	Err  error
}

func (InferiorOutputMsg) Type() string { return "InferiorOutputMsg" }

type ConsoleOutput struct{ Text string }
type TargetOutput struct{ Text string }
type LogOutput struct{ Text string }
type BreakpointHit struct{ Line int }
