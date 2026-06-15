package core

import "time"

// Event is the domain event interface for the legacy chat app and debugger backends.
// NewCGDB's terminal UI bus uses termui.Event instead.
type Event interface {
	Type() string
}

type SubmitMessage struct {
	Text string
	when time.Time
}

func (SubmitMessage) Type() string { return "SubmitMessage" }

type RunCommand struct {
	Command string
	when    time.Time
}

func (RunCommand) Type() string { return "RunCommand" }

type MessageSent struct {
	Text string
	when time.Time
}

func (MessageSent) Type() string { return "MessageSent" }

type Quit struct {
	Text string
}

func (Quit) Type() string { return "Quit" }

type GdbOutputMsg struct {
	Data string
	Err  error
}

func (GdbOutputMsg) Type() string { return "GdbOutputMsg" }

type ConsoleOutput struct{ Text string }
type TargetOutput struct{ Text string }
type LogOutput struct{ Text string }
type BreakpointHit struct{ Line int }
