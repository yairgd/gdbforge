package main

// breakCtl owns breakpoint send/sync/toggle intents. Wired as
// bpWidget.OnActivate = a.breaks.Activate (methods live on breakCtl, not DebuggerApp).
type breakCtl struct{ app *DebuggerApp }

// navCtl owns call-stack / thread activate intents.
type navCtl struct{ app *DebuggerApp }

// consoleCtl owns debugger console submit/interrupt/EOF/paint apply.
// Wired as wireConsole(..., a.console.onGdbConsoleSubmit, ...).
type consoleCtl struct{ app *DebuggerApp }

func (a *DebuggerApp) initControllers() {
	a.breaks.app = a
	a.nav.app = a
	a.console.app = a
}

