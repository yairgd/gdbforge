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

func (m SubmitMsg) Type() string     { return "SubmitMsg" }
func (m SubmitMsg) CommandID() CommandID { return m.CmdID }
