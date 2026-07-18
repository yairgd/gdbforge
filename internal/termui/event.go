package termui

type Event interface {
	Type() string
}

type CommandEvent interface {
	Event
	CommandID() CommandID
}

type Emitter func(Event)

type SubmitMsg struct {
	Text  string
	CmdID CommandID
	Args  string
}

func (m SubmitMsg) Type() string         { return "SubmitMsg" }
func (m SubmitMsg) CommandID() CommandID { return m.CmdID }

// CompletionMsg is published on platform.EventBus when Tab requests completions.
// CompletionBarWidget subscribes and shows a wildmenu row above the cmdline.
type CompletionMsg struct {
	Input string
	Token string
	Names []string
}

func (m CompletionMsg) Type() string { return "CompletionMsg" }

type BaseEvent struct {
	Cmd  Command
	Args []string
}

func (m BaseEvent) Type() string         { return "BaseEvent" }
func (m BaseEvent) CommandID() CommandID { return m.Cmd.ID }
