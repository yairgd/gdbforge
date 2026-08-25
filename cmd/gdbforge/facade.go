package main

// DebuggerApp is the UI/command wiring hub. Debugger policy lives on Backend
// (GDB vs Delve). List widgets take *DebuggerApp as host (ActivateBreakpoint,
// FocusCode, …). Prefer Backend methods over new isDLV() branches.
//
// Composition:
//   - LayoutShell — tab tree, pane marks, focus/swap policy
//   - DebugSession — backend, GDB widgets, debug controllers
//   - Cross-cutting — lua, search, serial, exec, keybindings, modes
//
// Controllers talk to the app through host interfaces (initControllers wires
// host = a). Orchestration (stop pipeline, mode dispatch) stays on DebuggerApp.
