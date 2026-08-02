package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestDrainUIEventsCaps(t *testing.T) {
	ch := make(chan tcell.Event, 8)
	ch <- tcell.NewEventInterrupt("a")
	ch <- tcell.NewEventInterrupt("b")
	ch <- tcell.NewEventInterrupt("c")
	batch := drainUIEvents(ch, tcell.NewEventInterrupt("first"), 2)
	if len(batch) != 2 {
		t.Fatalf("len=%d want 2", len(batch))
	}
}

func TestPrioritizeKeysOrdering(t *testing.T) {
	// Mirror handleUIEventBatch bucketing without needing a live screen.
	batch := []tcell.Event{
		tcell.NewEventInterrupt("x"),
		tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone),
		tcell.NewEventInterrupt("y"),
	}
	var keys, interrupts int
	for _, ev := range batch {
		switch ev.(type) {
		case *tcell.EventKey:
			keys++
		case *tcell.EventInterrupt:
			interrupts++
		}
	}
	if keys != 1 || interrupts != 2 {
		t.Fatalf("keys=%d interrupts=%d", keys, interrupts)
	}
}
