package main

// DebuggerApp is the UI/command wiring hub. Debugger policy lives on Backend
// (GDB vs Delve). List widgets take *DebuggerApp as host (ActivateBreakpoint,
// FocusCode, …). Prefer Backend methods over new isDLV() branches.
//
// Controllers that own domain logic (not thin forwarders): breakCtl, asmCtl,
// bufferCtl, debugInfoCtl, consoleCtl, inferiorIOCtl, completionCtl, searchCtl,
// luaCtl, dlvCtl. Each talks to the app through its own host interface
// (initControllers wires host = a). Orchestration (stop pipeline, layouts,
// mode dispatch) stays on DebuggerApp.
