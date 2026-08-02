package core

import "time"

// Drain reads and discards PTY messages for wait, or until the channel closes.
// Shared by gdb/dlv complete and MCP query helpers.
func Drain(ch <-chan PtyOutputMsg, wait time.Duration) {
	if ch == nil || wait <= 0 {
		return
	}
	deadline := time.After(wait)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			return
		}
	}
}
