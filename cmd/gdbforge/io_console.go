package main

import (
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

const (
	// Coalesce aggressively so a printf storm becomes ~20 UI events/sec, not thousands.
	inferiorOutputFlushInterval = 50 * time.Millisecond
	inferiorOutputFlushMaxBytes = 256 * 1024
	// Cap pending coalesced text; keep head + tail so Ctrl-C era context remains.
	inferiorPendingHardMax = 1024 * 1024
	// PostEvent backpressure: block (do not drop immediately) so the PTY reader stalls.
	inferiorPostRetry   = 5 * time.Millisecond
	inferiorPostRetries = 400 // ~2s before dropping a coalesced chunk
)

// inferiorHost is the narrow surface inferiorIOCtl needs from the composition
// root. DebuggerApp implements it; inferiorIOCtl must not depend on *DebuggerApp.
type inferiorHost interface {
	OutputWidget() *widgets.OutputWidget
	Screen() tcell.Screen
	State() *platform.AppState
	ConsoleSuspend()
	LogError(area, msg string)
	GDBWidget() *widgets.GDBWidget
	Debug() *debugstate.State
	RequestFrame()
}

// inferiorIOCtl owns the program stdio domain: the inferior PTY subscription,
// the IO pane wiring, and internal/external tty switches.
// DebuggerApp wires it; the ctl owns the domain.
type inferiorIOCtl struct {
	host      inferiorHost
	cancelSub func()
}

func (c *inferiorIOCtl) Register(bus *platform.EventBus) {
	platform.Subscribe(bus, c.onOutput)
}

func (c *inferiorIOCtl) onOutput(msg events.InferiorOutputMsg) {
	h := c.host
	if h == nil || msg.Data == "" {
		return
	}
	if out := h.OutputWidget(); out != nil {
		out.AppendInferior(msg.Data)
	}
	if gw := h.GDBWidget(); gw != nil && h.Debug().GdbTargetPrint() {
		gw.AppendTargetText(msg.Data)
	}
	h.RequestFrame()
}

// wire attaches the dedicated program PTY to the IO view and bridges stdout
// onto the UI event loop. The ctl owns Send / Subscribe.
func (c *inferiorIOCtl) wire(tty *ptyx.TTY) {
	h := c.host
	if h == nil || h.OutputWidget() == nil || tty == nil {
		return
	}
	out := h.OutputWidget()
	out.WireConsole(&widgets.ConsoleHandlers{
		Submit: func(line string) {
			c.send(tty, func() { _ = tty.Send(line) })
		},
		Interrupt: func() {
			c.send(tty, func() { _ = tty.SendRaw("\x03") })
		},
		Suspend: h.ConsoleSuspend,
		EOF: func() {
			c.send(tty, func() { _ = tty.SendRaw("\x04") })
		},
	})
	c.startBridge(tty)
	out.SetSizeFunc(tty.SetSize)
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

func (c *inferiorIOCtl) startBridge(tty *ptyx.TTY) {
	h := c.host
	if h == nil || tty == nil || h.Screen() == nil {
		return
	}
	c.stop()
	ch, cancel := tty.Subscribe()
	c.cancelSub = cancel
	screen := h.Screen()
	go coalesceInferiorOutput(ch, func(msg events.InferiorOutputMsg) {
		ev := tcell.NewEventInterrupt(msg)
		for i := 0; i < inferiorPostRetries; i++ {
			if err := screen.PostEvent(ev); err == nil {
				return
			}
			// Backpressure: stop draining the PTY subscriber until the UI accepts.
			time.Sleep(inferiorPostRetry)
		}
		h.LogError("inferior-io", "dropped inferior output (UI event queue full)")
	}, func() {
		_ = screen.PostEvent(tcell.NewEventInterrupt("inferior-exit"))
	})
}

// coalesceInferiorOutput batches PTY chunks so a busy UI event queue is less
// likely to drop program stdout (PostEvent returns ErrEventQFull under flood).
func coalesceInferiorOutput(ch <-chan core.PtyOutputMsg, post func(events.InferiorOutputMsg), onExit func()) {
	coalescePtyOutput(ch, ptyCoalesceOpts{
		Interval: inferiorOutputFlushInterval,
		MaxBytes: inferiorOutputFlushMaxBytes,
		HardMax:  inferiorPendingHardMax,
		Post: func(data string, err error) {
			if post == nil {
				return
			}
			post(events.InferiorOutputMsg{Data: data, Err: err})
		},
		OnExit: onExit,
	})
}

// stop drops the inferior PTY subscription (bridge teardown only).
func (c *inferiorIOCtl) stop() {
	if c.cancelSub != nil {
		c.cancelSub()
		c.cancelSub = nil
	}
}

// unwire stops reading the internal inferior PTY into the IO pane.
func (c *inferiorIOCtl) unwire() {
	c.stop()
	h := c.host
	if h == nil || h.OutputWidget() == nil {
		return
	}
	out := h.OutputWidget()
	out.WireConsole(nil)
	out.SetSizeFunc(nil)
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
	msg += " (TUI / real terminal — not this pane)\n"
	out.AppendInferior(msg)
}

// rewireInternal attaches the in-process inferior PTY to the IO pane.
func (c *inferiorIOCtl) rewireInternal(tty *ptyx.TTY) {
	h := c.host
	if tty == nil || h == nil || h.OutputWidget() == nil {
		return
	}
	c.unwire()
	c.wire(tty)
	h.OutputWidget().Clear()
	h.OutputWidget().AppendInferior("stdio: internal IO pane\n")
}
