package main

// Confirm facade — orthogonal mini-machine for quit / y-n gates.
// Mode stays Insert while typing y/n; Activity asks Confirm before a
// normal debugger interrupt when a gate is open.

// confirming is true while GDB QuitGate or Delve ConfirmGate awaits y/n.
func (a *DebuggerApp) confirming() bool {
	if a == nil {
		return false
	}
	if a.isDLV() {
		return a.dlv.confirm.Confirming()
	}
	if gb := a.gdbBackend(); gb != nil && gb.Client != nil {
		return gb.Client.Quit.Confirming()
	}
	return false
}

// onConfirmCtrlD starts quit (may open Confirm Asking).
func (a *DebuggerApp) onConfirmCtrlD() {
	if a == nil {
		return
	}
	a.console.onGdbConsoleEOF()
	a.requestFrameIfReady()
}

// onConfirmInterrupt handles Ctrl-C while Confirm is Asking.
// Delve [Y/n]?: send "n". GDB quit confirm: normal ^C (backend ignores flag).
func (a *DebuggerApp) onConfirmInterrupt() {
	if a == nil {
		return
	}
	a.console.onConfirmingInterrupt()
}
