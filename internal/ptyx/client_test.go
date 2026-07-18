package ptyx

import (
	"testing"

	"github.com/yairgd/cgdb-go/internal/core"
)

func TestFanOutDeliversToMultipleSubscribers(t *testing.T) {
	c := &Client{subs: make(map[chan core.PtyOutputMsg]struct{})}
	ch1, cancel1 := c.Subscribe()
	ch2, cancel2 := c.Subscribe()
	defer cancel1()
	defer cancel2()

	c.broadcast(core.PtyOutputMsg{Data: "hello"})

	got1 := <-ch1
	got2 := <-ch2
	if got1.Data != "hello" || got2.Data != "hello" {
		t.Fatalf("got %q and %q, want hello", got1.Data, got2.Data)
	}
}

func TestSubscribeCancelStopsDelivery(t *testing.T) {
	c := &Client{subs: make(map[chan core.PtyOutputMsg]struct{})}
	ch, cancel := c.Subscribe()
	cancel()

	_, ok := <-ch
	if ok {
		t.Fatal("expected closed channel after cancel")
	}

	c.broadcast(core.PtyOutputMsg{Data: "x"})
}
