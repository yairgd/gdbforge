package core

import "time"

//////////////////////////////
// Event (Domain Layer)
//////////////////////////////

type Event interface {
	Type() string
}

type CommandEvent interface {
	Event
	CommandID() CommandID
}

type Emitter func(Event)

// user typed message
type SubmitMessage struct {
	Text string
	when time.Time
}

func (SubmitMessage) Type() string { return "SubmitMessage" }

// command like :hello
type RunCommand struct {
	Command string
	when    time.Time
}

func (RunCommand) Type() string { return "RunCommand" }

// internal event (result)
type MessageSent struct {
	Text string
	when time.Time
}

func (MessageSent) Type() string { return "MessageSent" }

type SubmitMsg struct {
	Text  string
	CmdID CommandID
	Args  string
}

func (m SubmitMsg) Type() string            { return "SubmitMsg" }
func (m SubmitMsg) CommandID() CommandID { return m.CmdID }

// internal event (result)
type Quit struct {
	Text string
}

func (Quit) Type() string { return "Quit" }

// OutputMsg is sent to the UI layer
type GdbOutputMsg struct {
	Data string
	Err  error
}

func (GdbOutputMsg) Type() string { return "GdbOutputMsg" }

type ConsoleOutput struct{ Text string }
type TargetOutput struct{ Text string }
type LogOutput struct{ Text string }
type BreakpointHit struct{ Line int }
