package app

// Event represents a logical application event.
// It is intentionally abstract so different layers (TUI, Web, tests)
// can communicate with the core logic without depending on UI types.
type Event interface{}

// SubmitMessage represents sending a user message
// from the UI layer to the application core.
type SubmitMessage struct {
	Text string
}

// RunCommand represents execution of a command
// entered in the command line (e.g. :hello, :q).
type RunCommand struct {
	Command string
}

// ResizeEvent represents a window resize event.
// Useful if the core needs layout-aware behavior in the future.
type ResizeEvent struct {
	Width  int
	Height int
}
