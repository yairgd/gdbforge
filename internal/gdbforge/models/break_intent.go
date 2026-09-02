package models

// BreakIntentKind describes a breakpoint operation for the backend to execute.
type BreakIntentKind int

const (
	IntentInsert BreakIntentKind = iota
	IntentClear
	IntentDeleteByNumber
)

// BreakIntent is a semantic breakpoint action (no protocol command strings).
type BreakIntent struct {
	Kind   BreakIntentKind
	File   string
	Line   int
	Addr   string
	Number int
}
