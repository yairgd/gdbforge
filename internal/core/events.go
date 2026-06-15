package core

// Event is the domain event interface for debugger backends.
// cgdb-go's terminal UI bus uses termui.Event instead.
type Event interface {
	Type() string
}

type GdbOutputMsg struct {
	Data string
	Err  error
}

func (GdbOutputMsg) Type() string { return "GdbOutputMsg" }

type ConsoleOutput struct{ Text string }
type TargetOutput struct{ Text string }
type LogOutput struct{ Text string }
type BreakpointHit struct{ Line int }
