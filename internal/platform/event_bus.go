package platform

import "sync"

type EventBus struct {
	mu sync.RWMutex

	subs map[any][]func(any)
}

func NewEventBus() *EventBus {
	return &EventBus{
		subs: make(map[any][]func(any)),
	}
}

func Subscribe[T any](b *EventBus, h func(T)) {

	var key *T

	b.mu.Lock()
	defer b.mu.Unlock()

	b.subs[key] = append(b.subs[key], func(v any) {
		h(v.(T))
	})
}

func Publish[T any](b *EventBus, ev T) {

	var key *T

	b.mu.RLock()
	handlers := append([]func(any){}, b.subs[key]...)
	b.mu.RUnlock()

	for _, h := range handlers {
		h(ev)
	}
}
