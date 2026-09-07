// Package events holds debugger-app UI interrupt payloads.
package events

// GdbOutputMsg is an MI PTY chunk routed to consoleCtl for parsing (not GDB pane paint).
type GdbOutputMsg struct {
	Data string
	Err  error
}

func (GdbOutputMsg) Type() string { return "GdbOutputMsg" }
