package platform

import (
	"reflect"
	"sync"
)

// UIComponent registers typed event handlers on the bus at startup.
type UIComponent interface {
	Register(bus *EventBus)
}

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
	b.Dispatch(ev)
}

// Dispatch routes ev to all Subscribe handlers for ev's type. UI thread only.
func (b *EventBus) Dispatch(ev any) {
	if b == nil || ev == nil {
		return
	}
	key := handlerKey(ev)
	b.mu.RLock()
	handlers := append([]func(any){}, b.subs[key]...)
	b.mu.RUnlock()
	for _, h := range handlers {
		h(ev)
	}
}

func handlerKey(ev any) any {
	t := reflect.TypeOf(ev)
	if t.Kind() == reflect.Pointer {
		return reflect.Zero(t).Interface()
	}
	return reflect.Zero(reflect.PointerTo(t)).Interface()
}
