package main

import (
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/platform"
)

const (
	gdbOutputFlushInterval = 16 * time.Millisecond
	gdbOutputFlushMaxBytes = 64 * 1024
)

// startGdbConsoleBridge coalesces GDB PTY chunks onto the UI event loop.
func (a *DebuggerApp) startGdbConsoleBridge() {
	if a.gdbClient == nil || a.Screen() == nil {
		return
	}
	ch, cancel := a.gdbClient.Subscribe()
	a.gdbCancelSub = cancel
	screen := a.Screen()
	go coalesceGdbOutput(ch, func(msg core.GdbOutputMsg) {
		_ = screen.PostEvent(tcell.NewEventInterrupt(msg))
	}, func() {
		_ = screen.PostEvent(tcell.NewEventInterrupt("gdb-exit"))
	})
}

func coalesceGdbOutput(ch <-chan core.PtyOutputMsg, post func(core.GdbOutputMsg), onExit func()) {
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
		post(core.GdbOutputMsg{Data: data})
	}
	arm := func() {
		if flushTimer != nil {
			return
		}
		flushTimer = time.NewTimer(gdbOutputFlushInterval)
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
				post(core.GdbOutputMsg{Err: msg.Err})
				continue
			}
			if msg.Data == "" {
				continue
			}
			pending.WriteString(msg.Data)
			if pending.Len() >= gdbOutputFlushMaxBytes {
				flush()
			} else {
				arm()
			}
		case <-flushC:
			flush()
		}
	}
}

func (a *DebuggerApp) onGdbConsoleSubmit(raw string) {
	if a.gdbClient == nil || a.gdbWidget == nil {
		return
	}
	w := a.gdbWidget
	c := a.gdbClient

	cmd := raw
	if !c.Quit.Confirming() && cmd == "" {
		cmd = w.LastHistory()
	}

	if c.Quit.Confirming() {
		ans := strings.TrimSpace(strings.ToLower(raw))
		display := ans
		if display == "" {
			display = "n"
		}
		act := c.Quit.Answer(raw)
		if act == gdb.QuitReprompt {
			w.BeginLiveHost(gdb.QuitRepromptLines(), gdb.QuitConfirmHost)
			return
		}
		w.EchoSubmit(display)
		w.ClearInput()
		a.sendGdbQuitAction(act)
		w.FollowTailAndScroll()
		return
	}

	if act := c.Quit.SubmitQuitCommand(cmd); act != gdb.QuitNoop {
		if act == gdb.QuitShowConfirm {
			if cmd != "" {
				w.PushHistory(cmd)
				w.EchoSubmit(cmd)
			}
			w.BeginLiveHost(gdb.QuitConfirmLines(c.Quit.InferiorPID()), gdb.QuitConfirmHost)
			return
		}
		if cmd != "" {
			w.PushHistory(cmd)
			w.EchoSubmit(cmd)
		}
		w.ClearInput()
		a.sendGdbQuitAction(act)
		w.FollowTailAndScroll()
		return
	}

	if gdb.IsStackNavCmd(cmd) {
		a.pendingFrameSync = true
	}
	send := func() { _ = c.Send(cmd) }
	if cmd != "" {
		w.PushHistory(cmd)
		w.EchoSubmit(cmd)
	}
	a.withGdbUIOwner(send)
	w.ClearInput()
	w.FollowTailAndScroll()
}

func (a *DebuggerApp) onGdbConsoleInterrupt() {
	if a.gdbClient == nil || a.gdbWidget == nil {
		return
	}
	a.gdbWidget.ClearInput()
	a.withGdbUIOwner(func() { _ = a.gdbClient.Interrupt() })
}

func (a *DebuggerApp) onGdbConsoleEOF() {
	if a.gdbClient == nil {
		return
	}
	a.handleGdbQuitAction(a.gdbClient.RequestQuit(), "q")
}

func (a *DebuggerApp) handleGdbQuitAction(act gdb.QuitAction, echoCmd string) {
	if a.gdbClient == nil || a.gdbWidget == nil {
		return
	}
	w := a.gdbWidget
	switch act {
	case gdb.QuitShowConfirm:
		if echoCmd != "" {
			w.PushHistory(echoCmd)
			w.EchoSubmit(echoCmd)
		}
		w.BeginLiveHost(gdb.QuitConfirmLines(a.gdbClient.Quit.InferiorPID()), gdb.QuitConfirmHost)
	case gdb.QuitReprompt:
		w.BeginLiveHost(gdb.QuitRepromptLines(), gdb.QuitConfirmHost)
	default:
		w.FollowTailAndScroll()
	}
	a.sendGdbQuitAction(act)
}

func (a *DebuggerApp) sendGdbQuitAction(act gdb.QuitAction) {
	if a.gdbClient == nil || !act.Sends() {
		return
	}
	a.withGdbUIOwner(func() { _ = gdb.ApplyQuitAction(a.gdbClient, act) })
}

func (a *DebuggerApp) withGdbUIOwner(fn func()) {
	if a.State() != nil {
		a.State().WithPTYOwner(platform.PTYOwnerUI, fn)
	} else {
		fn()
	}
}

func (a *DebuggerApp) applyGdbMiUpdate(upd gdb.MiUpdate) {
	if a.gdbClient != nil {
		a.gdbClient.Quit.Observe(upd)
	}
	silent := a.State() != nil && a.State().SuppressGdbConsole()
	confirming := a.gdbClient != nil && a.gdbClient.Quit.Confirming()
	if !silent && a.gdbWidget != nil {
		includeTarget := a.State() != nil && a.State().GdbTargetPrint()
		a.gdbWidget.PaintMiDisplay(upd, confirming, includeTarget)
	}
	if upd.Stopped != nil {
		a.onGdbStopped(upd.Stopped)
	}
	if upd.InferiorExited {
		a.clearDebugInfoPanes()
	}
	// Wait for MI prompt after *stopped before -thread-info / -stack-list-frames.
	// Querying immediately races the stop reply and often captures an empty /
	// stale chunk — Threads pane then does not update until a later click.
	if a.pendingDebugInfo && upd.PromptReady {
		a.triggerPendingDebugInfo()
	}
	if a.pendingFrameSync {
		if upd.State == gdb.Error {
			a.pendingFrameSync = false
		} else if upd.PromptReady {
			a.pendingFrameSync = false
			a.onGdbFrameSync()
		}
	}
	if upd.State == gdb.Running {
		if a.State() != nil {
			a.State().SetInferiorRunning(true)
		}
	}
	if upd.BreakpointsChanged {
		a.onBreakpointsChanged()
	}
}

func (a *DebuggerApp) handleGdbOutputMsg(msg core.GdbOutputMsg) {
	if msg.Data == "" || a.gdbInputState == nil {
		return
	}
	upd := a.gdbInputState.PushRaw(msg.Data)
	a.applyGdbMiUpdate(upd)
}
