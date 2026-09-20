package app

import (
	"sync"
	"testing"
	"time"

	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/termforge/ptyx"
)

func TestCoalesceGdbOutputBatchesChunks(t *testing.T) {
	ch := make(chan ptyx.PtyOutputMsg, 8)
	var mu sync.Mutex
	var got []events.GdbOutputMsg
	done := make(chan struct{})

	go coalesceGdbOutput(ch, func(msg events.GdbOutputMsg) {
		mu.Lock()
		got = append(got, msg)
		mu.Unlock()
	}, func() { close(done) })

	ch <- ptyx.PtyOutputMsg{Data: "a"}
	ch <- ptyx.PtyOutputMsg{Data: "b"}
	ch <- ptyx.PtyOutputMsg{Data: "c"}
	close(ch)

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("want 1 coalesced msg, got %d: %#v", len(got), got)
	}
	if got[0].Data != "abc" {
		t.Fatalf("data=%q", got[0].Data)
	}
}
