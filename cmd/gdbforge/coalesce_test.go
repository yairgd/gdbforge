package main

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestCoalesceRunnerTrailingRerun(t *testing.T) {
	var r coalesceRunner
	var n atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})

	r.Schedule(func() {
		n.Add(1)
		select {
		case started <- struct{}{}:
		default:
		}
		if n.Load() == 1 {
			<-release
		}
	})
	<-started
	r.Schedule(func() {}) // marks pending; same work re-runs
	close(release)

	deadline := time.After(500 * time.Millisecond)
	for n.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("n=%d want >= 2", n.Load())
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}
