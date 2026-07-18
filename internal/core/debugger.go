package core

type Debugger interface {
	Send(cmd string) error
	SendRaw(raw string) error
}

// Session is a Debugger that owns process lifetime (Close).
// External APIs (e.g. MCP) should use Session, not a concrete backend type.
type Session interface {
	Debugger
	Close()
}
