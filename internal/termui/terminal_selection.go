package termui

// TerminalSelectionPane is implemented by CompositeTerminal-backed widgets.
type TerminalSelectionPane interface {
	HasTerminalSelection() bool
}
