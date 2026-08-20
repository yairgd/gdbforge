package widgets

import "github.com/yairgd/gdbforge/internal/termui"

// ConsoleHandlers are app-owned intents for a ConsolePane widget.
// Pass nil to WireConsole to detach / clear.
type ConsoleHandlers struct {
	Submit    func(cmd string)
	Interrupt func()
	Suspend   func()
	EOF       func()
}

func clearConsoleIntents(p *termui.ConsolePane) {
	if p == nil {
		return
	}
	p.OnSubmit = nil
	p.OnInterrupt = nil
	p.OnEOF = nil
	p.OnSuspend = nil
}

func bindConsoleIntents(
	p *termui.ConsolePane,
	submit func(string),
	interrupt, eof, suspend func(),
) {
	if p == nil {
		return
	}
	p.OnSubmit = submit
	p.OnInterrupt = interrupt
	p.OnEOF = eof
	p.OnSuspend = suspend
}
