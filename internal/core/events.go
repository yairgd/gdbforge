package core

//////////////////////////////
// Event (Domain Layer)
//////////////////////////////

type Event interface {
	Type() string
}
type Emitter func(Event)

// user typed message
type SubmitMessage struct {
	Text string
}

func (SubmitMessage) Type() string { return "SubmitMessage" }

// command like :hello
type RunCommand struct {
	Command string
}

func (RunCommand) Type() string { return "RunCommand" }

// internal event (result)
type MessageSent struct {
	Text string
}

func (MessageSent) Type() string { return "MessageSent" }

// internal event (result)
type SubmitMsg struct {
	Text string
}

func (SubmitMsg) Type() string { return "SubmitMsg" }

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

//////////////////////////////
// Helpers
//////////////////////////////
