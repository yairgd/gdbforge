package main

import (
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

const (
	inferiorOutputFlushInterval = 16 * time.Millisecond
	inferiorOutputFlushMaxBytes = 64 * 1024
	inferiorPostRetry           = 50 * time.Millisecond
	inferiorPostRetries         = 40 // ~2s before dropping a coalesced chunk
)

// wireInferiorIO attaches the dedicated program PTY to the IO view and
// bridges stdout onto the UI event loop. App owns Send / Subscribe.
func (a *DebuggerApp) wireInferiorIO(tty *ptyx.TTY) {
	if a.outputWidget == nil || tty == nil {
		return
	}
	a.outputWidget.EnableInput(true)
	a.outputWidget.SetSizeFunc(tty.SetSize)
	a.outputWidget.SetOnSubmit(func(line string) {
		a.sendInferior(tty, func() { _ = tty.Send(line) })
	})
	a.outputWidget.SetOnInterrupt(func() {
		a.sendInferior(tty, func() { _ = tty.SendRaw("\x03") })
	})
	a.outputWidget.SetOnSuspend(func() {
		// Ctrl-Z on the program's terminal → SIGTSTP via ^Z on the inferior PTY.
		a.sendInferior(tty, func() { _ = tty.SendRaw("\x1a") })
	})
	a.outputWidget.SetOnEOF(func() {
		a.sendInferior(tty, func() { _ = tty.SendRaw("\x04") })
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
	go coalesceInferiorOutput(ch, func(msg core.InferiorOutputMsg) {
		ev := tcell.NewEventInterrupt(msg)
		for i := 0; i < inferiorPostRetries; i++ {
			if err := screen.PostEvent(ev); err == nil {
				return
			}
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
func coalesceInferiorOutput(ch <-chan core.PtyOutputMsg, post func(core.InferiorOutputMsg), onExit func()) {
	var pending strings.Builder
	var flushTimer *time.Timer
	var flushC <-chan time.Time

	disarm := func() {
		if flushTimer == nil {
			return
		}
		if !flushTimer.Stop() {
			select {
			case <-flushTimer.C:
			default:
			}
		}
		flushTimer = nil
		flushC = nil
	}
	flush := func() {
		disarm()
		if pending.Len() == 0 {
			return
		}
		data := pending.String()
		pending.Reset()
		post(core.InferiorOutputMsg{Data: data})
	}
	arm := func() {
		if flushTimer != nil {
			return
		}
		flushTimer = time.NewTimer(inferiorOutputFlushInterval)
		flushC = flushTimer.C
	}

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				flush()
				if onExit != nil {
					onExit()
				}
				return
			}
			if msg.Err != nil {
				flush()
				post(core.InferiorOutputMsg{Err: msg.Err})
				continue
			}
			if msg.Data == "" {
				continue
			}
			pending.WriteString(msg.Data)
			if pending.Len() >= inferiorOutputFlushMaxBytes {
				flush()
			} else {
				arm()
			}
		case <-flushC:
			flush()
		}
	}
}

func (a *DebuggerApp) stopInferiorIO() {
	if a.inferiorCancelSub != nil {
		a.inferiorCancelSub()
		a.inferiorCancelSub = nil
	}
}
