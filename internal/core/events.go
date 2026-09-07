package core

// Event is the domain event interface for debugger backends.
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
