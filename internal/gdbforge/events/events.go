// Package events holds debugger-app UI interrupt payloads.
package events

// GdbOutputMsg is a UI-routed debugger-console PTY chunk (EventInterrupt → GDBWidget).
type GdbOutputMsg struct {
	Data string
	Err  error
}

func (GdbOutputMsg) Type() string { return "GdbOutputMsg" }

// InferiorOutputMsg is a UI-routed chunk from the debugged program's PTY
// (EventInterrupt → IO / OutputWidget), after -inferior-tty-set.
type InferiorOutputMsg struct {
	Data string
	Err  error
}

func (InferiorOutputMsg) Type() string { return "InferiorOutputMsg" }
