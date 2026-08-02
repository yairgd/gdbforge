package widgets

// MiPaintUpdate is the paint-only view of an MI update (no gdb import).
type MiPaintUpdate struct {
	DisplayLines []string
	TargetLines  []string
	PromptReady  bool
	PromptLine   string
}

// QuitConfirmHost is the live input line CLI GDB uses for quit confirmation.
const QuitConfirmHost = "Quit anyway? (y or n) "

// QuitConfirmLines returns the scrollback block CLI GDB prints before
// QuitConfirmHost when an inferior is alive.
func QuitConfirmLines(pid string) []string {
	if pid == "" {
		pid = "?"
	}
	return []string{
		"A debugging session is active.",
		"",
		"\tInferior 1 [process " + pid + "] will be killed.",
		"",
	}
}
