package main

// consoleWire is the SetOn* surface shared by GDB, IO, and Exec panes.
type consoleWire interface {
	SetOnSubmit(fn func(cmd string))
	SetOnInterrupt(fn func())
	SetOnSuspend(fn func())
	SetOnEOF(fn func())
}

// consoleHandlers are the app-owned intents for a console pane.
type consoleHandlers struct {
	Submit    func(cmd string)
	Interrupt func()
	Suspend   func()
	EOF       func()
}

// wireConsole attaches submit/interrupt/suspend/EOF handlers in one place.
func wireConsole(w consoleWire, h consoleHandlers) {
	if w == nil {
		return
	}
	w.SetOnSubmit(h.Submit)
	w.SetOnInterrupt(h.Interrupt)
	w.SetOnSuspend(h.Suspend)
	w.SetOnEOF(h.EOF)
}

// unwireConsole clears console intents (e.g. when detaching inferior IO).
func unwireConsole(w consoleWire) {
	wireConsole(w, consoleHandlers{})
}
