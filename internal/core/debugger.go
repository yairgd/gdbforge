package core

import "context"

type Debugger interface {
	Send(cmd string) error
	SendRaw(raw string) error
}

// PTYWriter is the exclusive write handle granted by Session.WithWrite.
// Callers must not use it after WithWrite returns.
type PTYWriter interface {
	Send(cmd string) error
	SendRaw(raw string) error
}

// Session is a Debugger that owns process lifetime (Close) and supports
// multi-reader PTY output via Subscribe plus exclusive write via WithWrite.
// External APIs (MCP, REST, in-app AI) should use Session — not a concrete
// backend type — and must not Close the session while the UI owns it.
type Session interface {
	Debugger
	Close()
	// Subscribe fans out raw PTY output. cancel unregisters; Close
	// closes every subscription. Drain promptly — a full buffer drops
	// chunks for that subscriber only.
	Subscribe() (ch <-chan PtyOutputMsg, cancel func())
	// WithWrite runs fn while holding the exclusive PTY write lock so only
	// one writer (UI or MCP) is active. Readers (Subscribe) are unaffected.
	WithWrite(ctx context.Context, fn func(w PTYWriter) error) error
}
