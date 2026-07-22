package main

import (
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/platform"
)

const (
	gdbOutputFlushInterval = 16 * time.Millisecond
	gdbOutputFlushMaxBytes = 64 * 1024
)

// startGdbConsoleBridge coalesces debugger PTY chunks onto the UI event loop.
func (a *DebuggerApp) startGdbConsoleBridge() {
	sess := a.GDB()
	if sess == nil || a.Screen() == nil {
		return
	}
	ch, cancel := sess.Subscribe()
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
	if a.gdbWidget == nil || a.GDB() == nil {
		return
	}
	if a.isDLV() {
		a.onDlvConsoleSubmit(raw)
		return
	}
	if a.gdbClient == nil {
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
	// Run-control via MI so the GDB pane does not dump CLI source/line listings
	// (Code widget already follows *stopped).
	sendCmd := gdb.CLIExecToMI(cmd)
	send := func() { _ = c.Send(sendCmd) }
	if cmd != "" {
		w.PushHistory(cmd)
		w.EchoSubmit(cmd)
	}
	a.withGdbUIOwner(send)
	w.ClearInput()
	w.FollowTailAndScroll()
}

func (a *DebuggerApp) onDlvConsoleSubmit(raw string) {
	if a.dlvClient == nil || a.gdbWidget == nil {
		return
	}
	w := a.gdbWidget
	c := a.dlvClient

	cmd := raw
	if cmd == "" {
		cmd = w.LastHistory()
	}
	if dlv.IsStackNavCmd(cmd) {
		a.pendingFrameSync = true
	}
	if isDlvRunCmd(cmd) {
		if a.State() != nil {
			a.State().SetInferiorRunning(true)
		}
	}
	// Keep Delve CLI as-is (no MI mapping).
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
	if a.gdbWidget == nil || a.GDB() == nil {
		return
	}
	a.gdbWidget.ClearInput()
	if a.isDLV() && a.dlvClient != nil {
		a.withGdbUIOwner(func() { _ = a.dlvClient.Interrupt() })
		return
	}
	if a.gdbClient != nil {
		a.withGdbUIOwner(func() { _ = a.gdbClient.Interrupt() })
	}
}

func (a *DebuggerApp) onGdbConsoleEOF() {
	if a.isDLV() {
		if a.dlvClient == nil {
			return
		}
		// Delve: send quit; it may ask for confirmation interactively.
		w := a.gdbWidget
		if w != nil {
			w.PushHistory("quit")
			w.EchoSubmit("quit")
			w.ClearInput()
		}
		a.withGdbUIOwner(func() { _ = a.dlvClient.Send("quit") })
		return
	}
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
	a.applyStopAndPromptSideEffects(upd.Stopped, upd.InferiorExited, upd.PromptReady, upd.State, upd.BreakpointsChanged)
}

func (a *DebuggerApp) applyDlvUpdate(upd dlv.Update) {
	silent := a.State() != nil && a.State().SuppressGdbConsole()
	if !silent && a.gdbWidget != nil {
		a.gdbWidget.PaintDlvDisplay(upd.DisplayLines, upd.PromptReady, upd.PromptLine)
	}
	a.applyStopAndPromptSideEffects(upd.Stopped, upd.InferiorExited, upd.PromptReady, upd.State, upd.BreakpointsChanged)
}

func (a *DebuggerApp) applyStopAndPromptSideEffects(
	stopped *gdb.MiStopMsg,
	inferiorExited bool,
	promptReady bool,
	state gdb.GdbState,
	breakpointsChanged bool,
) {
	if stopped != nil {
		a.onGdbStopped(stopped)
	}
	if inferiorExited {
		a.clearDebugInfoPanes()
	}
	if a.pendingDebugInfo && promptReady {
		a.triggerPendingDebugInfo()
	}
	if a.pendingFrameSync {
		if state == gdb.Error {
			a.pendingFrameSync = false
		} else if promptReady {
			a.pendingFrameSync = false
			a.onGdbFrameSync()
		}
	}
	if state == gdb.Running {
		if a.State() != nil {
			a.State().SetInferiorRunning(true)
		}
	}
	if breakpointsChanged {
		a.onBreakpointsChanged()
	}
}

// handleDebuggerOutputMsg routes coalesced PTY output to the active backend parser.
func (a *DebuggerApp) handleDebuggerOutputMsg(msg core.GdbOutputMsg) {
	if msg.Data == "" {
		return
	}
	if a.isDLV() {
		if a.dlvInputState == nil {
			return
		}
		a.applyDlvUpdate(a.dlvInputState.PushRaw(msg.Data))
		return
	}
	if a.gdbInputState == nil {
		return
	}
	a.applyGdbMiUpdate(a.gdbInputState.PushRaw(msg.Data))
}

// handleGdbOutputMsg is kept as an alias for existing call sites / tests.
func (a *DebuggerApp) handleGdbOutputMsg(msg core.GdbOutputMsg) {
	a.handleDebuggerOutputMsg(msg)
}

func isDlvRunCmd(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	switch strings.Fields(cmd)[0] {
	case "c", "continue", "n", "next", "s", "step", "stepout", "finish", "nexti", "ni", "stepi", "si":
		return true
	default:
		return false
	}
}
