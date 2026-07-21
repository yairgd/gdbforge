package main

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
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
	go func() {
		for msg := range ch {
			_ = screen.PostEvent(tcell.NewEventInterrupt(core.InferiorOutputMsg{
				Data: msg.Data,
				Err:  msg.Err,
			}))
		}
	}()
}

func (a *DebuggerApp) stopInferiorIO() {
	if a.inferiorCancelSub != nil {
		a.inferiorCancelSub()
		a.inferiorCancelSub = nil
	}
}
