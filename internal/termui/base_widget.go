package termui

const (
	CmdPushData CommandID = iota + 10
)

type BaseWidget struct {
	Events chan Event // outbound -> app
	Inbox  chan Event // inbound  <- app
}

func NewBaseWidget() BaseWidget {
	return BaseWidget{
		Events: make(chan Event, 16),
		Inbox:  make(chan Event, 16),
	}
}

func (b *BaseWidget) Emit(ev Event) {
	if b.Events != nil {
		b.Events <- ev
	}
}

func (b *BaseWidget) Start(handler func(Event)) {
	go func() {
		for ev := range b.Inbox {
			handler(ev)
		}
	}()
}

// send event into the widget
func (b *BaseWidget) Post(cmd Command, args ...string) {
	if b.Events != nil {
		b.Events <- BaseEvent{
			Cmd:  cmd,
			Args: args,
		}
	}
}
