package widgets

import (
	"sync"
	"testing"
	"time"

	"github.com/yairgd/cgdb-go/internal/core"
)

func TestCoalesceGdbOutputBatchesChunks(t *testing.T) {
	ch := make(chan core.PtyOutputMsg, 8)
	var mu sync.Mutex
	var got []core.GdbOutputMsg
	done := make(chan struct{})

	go coalesceGdbOutput(ch, func(msg core.GdbOutputMsg) {
		mu.Lock()
		got = append(got, msg)
		mu.Unlock()
	}, func() { close(done) })

	ch <- core.PtyOutputMsg{Data: "a"}
	ch <- core.PtyOutputMsg{Data: "b"}
	ch <- core.PtyOutputMsg{Data: "c"}
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
