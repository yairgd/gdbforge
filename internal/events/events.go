package events

//////////////////////////////
// Event (Domain Layer)
//////////////////////////////

type Event interface {
	Type() string
}

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

//////////////////////////////
// Helpers
//////////////////////////////
