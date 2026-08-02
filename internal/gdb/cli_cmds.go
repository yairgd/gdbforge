package gdb

import "strings"

// IsStackNavCmd reports CLI/MI commands that change the selected stack frame
// without a *stopped event (frame / f / up / down / -stack-select-frame).
func IsStackNavCmd(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "frame", "f", "up", "down", "select-frame":
		return true
	}
	return strings.HasPrefix(fields[0], "-stack-select-frame")
}

// StopNeedsUIRefresh is true for *stopped reasons that should update Code /
// threads / call stack (not inferior-exit reasons).
func StopNeedsUIRefresh(stop *MiStopMsg) bool {
	if stop == nil {
		return false
	}
	switch stop.Reason {
	case "exited-normally", "exited", "exited-signalled":
		return false
	default:
		return true
	}
}
