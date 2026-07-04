package platform

type Event interface{}

type Handler func(Event)

type EventBus struct {
	handlers []Handler
}

func NewEventBus() *EventBus {
	return &EventBus{}
}

func (b *EventBus) Subscribe(h Handler) {
	b.handlers = append(b.handlers, h)
}

func (b *EventBus) Publish(ev Event) {
	for _, h := range b.handlers {
		h(ev)
	}
}
