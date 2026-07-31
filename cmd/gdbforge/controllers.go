package main

// breakCtl owns breakpoint send/sync helpers still shared by CodeWidget and
// domain paths (syncBreakpointViews, sendBreakpointCmd, …). List-widget
// intents (ToggleBreakpoint, ActivateBreakpoint, …) live on *DebuggerApp.
type breakCtl struct{ app *DebuggerApp }

// navCtl is reserved for navigation helpers; ActivateThread / ActivateCallStack
// live on *DebuggerApp.
type navCtl struct{ app *DebuggerApp }

// consoleCtl owns debugger console submit/interrupt/EOF/paint apply.
// Wired as wireConsole(..., a.console.onGdbConsoleSubmit, ...).
type consoleCtl struct{ app *DebuggerApp }

func (a *DebuggerApp) initControllers() {
	a.breaks.app = a
	a.nav.app = a
	a.console.app = a
}

