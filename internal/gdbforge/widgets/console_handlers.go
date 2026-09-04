package widgets

// ConsoleHandlers are app-owned intents for the Lua REPL widget.
// Pass nil to WireConsole to detach / clear.
type ConsoleHandlers struct {
	Submit    func(cmd string)
	Interrupt func()
	Suspend   func()
	EOF       func()
}
