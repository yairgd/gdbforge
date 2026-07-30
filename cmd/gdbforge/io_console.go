package main

import (
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
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

// wireInferiorIO attaches the dedicated program PTY to the IO view and
// bridges stdout onto the UI event loop. App owns Send / Subscribe.
func (a *DebuggerApp) wireInferiorIO(tty *ptyx.TTY) {
	if a.outputWidget == nil || tty == nil {
		return
	}
	a.outputWidget.EnableInput(true)
	a.outputWidget.SetSizeFunc(tty.SetSize)
	wireConsole(a.outputWidget, consoleHandlers{
		Submit: func(line string) {
			a.sendInferior(tty, func() { _ = tty.Send(line) })
		},
		Interrupt: func() {
			a.sendInferior(tty, func() { _ = tty.SendRaw("\x03") })
		},
		Suspend: a.onGdbConsoleSuspend,
		EOF: func() {
			a.sendInferior(tty, func() { _ = tty.SendRaw("\x04") })
		},
	})
	a.startInferiorIOBridge(tty)
}

func (a *DebuggerApp) sendInferior(tty *ptyx.TTY, send func()) {
	if tty == nil || send == nil {
		return
	}
	if st := a.State(); st != nil {
		st.WithPTYOwner(platform.PTYOwnerUI, send)
		return
	}
	send()
}

func (a *DebuggerApp) startInferiorIOBridge(tty *ptyx.TTY) {
	if tty == nil || a.Screen() == nil {
		return
	}
	if a.inferiorCancelSub != nil {
		a.inferiorCancelSub()
		a.inferiorCancelSub = nil
	}
	ch, cancel := tty.Subscribe()
	a.inferiorCancelSub = cancel
	screen := a.Screen()
	log := a.ctx.Log.Named("inferior-io")
	go coalesceInferiorOutput(ch, func(msg events.InferiorOutputMsg) {
		ev := tcell.NewEventInterrupt(msg)
		for i := 0; i < inferiorPostRetries; i++ {
			if err := screen.PostEvent(ev); err == nil {
				return
			}
			// Backpressure: stop draining the PTY subscriber until the UI accepts.
			time.Sleep(inferiorPostRetry)
		}
		if log != nil {
			log.Error("dropped inferior output (UI event queue full)")
		}
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

func (a *DebuggerApp) stopInferiorIO() {
	if a.inferiorCancelSub != nil {
		a.inferiorCancelSub()
		a.inferiorCancelSub = nil
	}
}
