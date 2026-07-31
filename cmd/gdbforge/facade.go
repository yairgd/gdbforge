package main

// DebuggerApp is the UI/command wiring hub. Debugger policy lives on Backend
// (GDB vs Delve). List widgets take *DebuggerApp as host (ActivateBreakpoint,
// FocusCode, …). Prefer Backend methods over new isDLV() branches.
