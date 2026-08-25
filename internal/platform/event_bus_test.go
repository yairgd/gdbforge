package platform

import "testing"

type dispatchTestEv struct{ N int }

type dispatchNameEv struct{ Name string }

func TestEventBusDispatch(t *testing.T) {
	bus := NewEventBus()
	var got int
	Subscribe(bus, func(ev dispatchTestEv) {
		got = ev.N
	})
	bus.Dispatch(dispatchTestEv{N: 42})
	if got != 42 {
		t.Fatalf("got=%d want 42", got)
	}
}

func TestPublishDelegatesToDispatch(t *testing.T) {
	bus := NewEventBus()
	var name string
	Subscribe(bus, func(ev dispatchNameEv) {
		name = ev.Name
	})
	Publish(bus, dispatchNameEv{Name: "ok"})
	if name != "ok" {
		t.Fatalf("name=%q", name)
	}
}
