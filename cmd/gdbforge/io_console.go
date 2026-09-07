package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
	"github.com/yairgd/gdbforge/internal/termui"
)

// inferiorHost is the narrow surface inferiorIOCtl needs from the composition
// root. DebuggerApp implements it; inferiorIOCtl must not depend on *DebuggerApp.
type inferiorHost interface {
	OutputWidget() *widgets.OutputWidget
	State() *platform.AppState
	GDBWidget() *widgets.GDBWidget
	Debug() *debugstate.State
	RequestFrame()
}

// inferiorIOCtl owns the program stdio domain: the inferior PTY subscription,
// the IO pane wiring, and internal/external tty switches.
// DebuggerApp wires it; the ctl owns the domain.
type inferiorIOCtl struct {
	host inferiorHost
}

func (c *inferiorIOCtl) wire(tty *ptyx.TTY) {
	h := c.host
	if h == nil || h.OutputWidget() == nil || tty == nil {
		return
	}
	opts := termui.WireTTYOpts{PostFrame: h.RequestFrame}
	if h.Debug() != nil {
		opts.OnData = func(data string) {
			if gw := h.GDBWidget(); gw != nil && h.Debug().GdbTargetPrint() {
				gw.AppendTargetText(data)
			}
		}
	}
	h.OutputWidget().WireInferiorOpts(tty, opts)
}

func (c *inferiorIOCtl) send(tty *ptyx.TTY, send func()) {
	if tty == nil || send == nil {
		return
	}
	if h := c.host; h != nil {
		if st := h.State(); st != nil {
			st.WithPTYOwner(platform.PTYOwnerUI, send)
			return
		}
	}
	send()
}

// unwire stops reading the PTY into the IO pane.
func (c *inferiorIOCtl) unwire() {
	h := c.host
	if h == nil || h.OutputWidget() == nil {
		return
	}
	h.OutputWidget().Detach()
}

// markExternal clears the IO pane and notes that stdio is on an external tty.
func (c *inferiorIOCtl) markExternal(path string) {
	c.unwire()
	h := c.host
	if h == nil || h.OutputWidget() == nil {
		return
	}
	out := h.OutputWidget()
	out.Clear()
	msg := "stdio: external tty"
	if path != "" {
		msg += " " + path
	}
	msg += " (TUI / real terminal — not this pane)"
	out.AppendHostLine(msg)
}

// rewireInternal attaches the in-process inferior PTY to the IO pane.
func (c *inferiorIOCtl) rewireInternal(tty *ptyx.TTY) {
	h := c.host
	if tty == nil || h == nil || h.OutputWidget() == nil {
		return
	}
	c.unwire()
	out := h.OutputWidget()
	out.Clear()
	c.wire(tty)
	out.AppendHostLine("stdio: internal IO pane")
}

// restoreInferiorIO re-attaches the debug session inferior PTY after serial console teardown.
func (a *DebuggerApp) restoreInferiorIO() {
	a.syncInferiorIOView()
}

// wireSerialConsole attaches the UART console leg to the IO pane (in-app minicom).
func (a *DebuggerApp) wireSerialConsole() error {
	tty, err := a.serial.TermTTY()
	if err != nil || tty == nil {
		return err
	}
	a.inferiorIO.unwire()
	if a.outputWidget != nil {
		a.outputWidget.Clear()
	}
	a.inferiorIO.wire(tty)
	if a.outputWidget != nil {
		a.outputWidget.AppendHostLine("serial: console on " + a.serial.Device() + " (IO pane)")
	}
	if a.TermApp != nil {
		a.RequestFrame()
	}
	return nil
}
