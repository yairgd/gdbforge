package main

// DebuggerApp is the UI/command wiring hub. Debugger policy lives on Backend
// (GDB vs Delve). Breakpoint / stack / thread intents live on breakCtl / navCtl
// (e.g. bpWidget.OnActivate = a.breaks.Activate). Prefer Backend / controller
// methods over new isDLV() branches or new fat DebuggerApp methods.
