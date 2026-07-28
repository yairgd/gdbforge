package main

import (
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
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
	gen := a.gdbBridgeGen.Add(1)
	ch, cancel := sess.Subscribe()
	a.gdbCancelSub = cancel
	screen := a.Screen()
	go coalesceGdbOutput(ch, func(msg events.GdbOutputMsg) {
		_ = screen.PostEvent(tcell.NewEventInterrupt(msg))
	}, func() {
		// Ignore closes from an old bridge (e.g. Delve --tty restart).
		if a.gdbBridgeGen.Load() != gen {
			return
		}
		_ = screen.PostEvent(tcell.NewEventInterrupt("gdb-exit"))
	})
}

func coalesceGdbOutput(ch <-chan core.PtyOutputMsg, post func(events.GdbOutputMsg), onExit func()) {
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
		post(events.GdbOutputMsg{Data: data})
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
				post(events.GdbOutputMsg{Err: msg.Err})
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
		a.codeNavGen++
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

	// Answer Delve [Y/n]? without treating the reply as a new CLI command.
	if a.dlvConfirm.Confirming() {
		send := func() { _ = c.Send(cmd) }
		if cmd != "" {
			w.EchoSubmit(cmd)
		}
		a.withGdbUIOwner(send)
		w.ClearInput()
		w.FollowTailAndScroll()
		return
	}

	if dlv.IsStackNavCmd(cmd) {
		a.codeNavGen++
		a.dlvSuppressStopUI++
		a.pendingFrameSync = true
		cur := 0
		if a.callstackWidget != nil {
			if fr, ok := a.callstackWidget.SelectedFrame(); ok {
				cur = fr.Level
			}
		}
		if level, ok := dlv.FrameNavTargetLevel(cmd, cur); ok {
			a.noteFrameSyncLevel(level)
		}
	}
	if isDlvRunCmd(cmd) {
		if a.State() != nil {
			a.Debug().SetInferiorRunning(true)
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
	if a.gdbWidget != nil {
		a.gdbWidget.ClearInput()
	}
	if a.GDB() == nil {
		return
	}
	// Interrupt must not wait on PTY-owner bookkeeping: GDB/Delve only leave
	// continue via ^C/SIGINT (typed commands sit unread until the prompt returns).
	if a.isDLV() && a.dlvClient != nil {
		// At a Delve yes/no prompt, cancel with "n" — do not SIGINT-spam dlv.
		if a.dlvConfirm.Confirming() {
			a.withGdbUIOwner(func() { _ = a.dlvClient.Send("n") })
			return
		}
		running := a.State() != nil && a.Debug().InferiorRunning()
		if !running {
			return
		}
		_ = a.dlvClient.Interrupt()
		a.RequestFrame()
		return
	}
	if a.gdbClient != nil {
		_ = a.gdbClient.Interrupt()
		a.RequestFrame()
	}
}

// onGdbConsoleSuspend handles Ctrl-Z like GDB: SIGTSTP the inferior while it
// is running; otherwise suspend gdbforge (job control, shell `fg` to resume).
func (a *DebuggerApp) onGdbConsoleSuspend() {
	running := a.State() != nil && a.Debug().InferiorRunning()
	if running {
		if a.gdbClient != nil {
			a.withGdbUIOwner(func() { _ = a.gdbClient.SuspendInferior() })
			return
		}
		// Delve: no MI pid tracking yet — ^Z on the inferior TTY (cooked mode).
		if tty := a.inferiorTTY(); tty != nil {
			a.sendInferior(tty, func() { _ = tty.SendRaw("\x1a") })
			return
		}
	}
	a.Suspend()
}

func (a *DebuggerApp) inferiorTTY() *ptyx.TTY {
	if a.gdbClient != nil {
		return a.gdbClient.InferiorTTY()
	}
	if a.dlvClient != nil {
		return a.dlvClient.InferiorTTY()
	}
	return nil
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
	silent := a.State() != nil && a.Debug().SuppressGdbConsole()
	confirming := a.gdbClient != nil && a.gdbClient.Quit.Confirming()
	if !silent && a.gdbWidget != nil {
		includeTarget := a.State() != nil && a.Debug().GdbTargetPrint()
		a.gdbWidget.PaintMiDisplay(widgets.MiPaintUpdate{
			DisplayLines: upd.DisplayLines,
			TargetLines:  upd.TargetLines,
			PromptReady:  upd.PromptReady,
			PromptLine:   upd.PromptLine,
		}, confirming, includeTarget)
	}
	a.applyStopAndPromptSideEffects(upd.Stopped, upd.InferiorExited, upd.PromptReady, upd.State, upd.BreakpointsChanged)
}

func (a *DebuggerApp) applyDlvUpdate(upd dlv.Update) {
	silent := a.State() != nil && a.Debug().SuppressGdbConsole()
	a.dlvConfirm.Observe(upd)
	confirming := a.dlvConfirm.Confirming()
	if !silent && a.gdbWidget != nil {
		a.gdbWidget.PaintDlvDisplay(upd.DisplayLines, upd.PromptReady, upd.PromptLine, confirming)
		if upd.ConfirmReady {
			host := upd.ConfirmHost
			if host == "" {
				host = a.dlvConfirm.Host()
			}
			a.gdbWidget.BeginLiveHost(nil, host)
		}
	}
	// Defer BP list Query while Delve waits for y/n (Query would steal the answer).
	bpChanged := upd.BreakpointsChanged
	if bpChanged && confirming {
		a.dlvBPDeferred = true
		bpChanged = false
	}
	a.applyStopAndPromptSideEffects(upd.Stopped, upd.InferiorExited, upd.PromptReady, upd.State, bpChanged)
	if upd.PromptReady && a.dlvBPDeferred {
		a.dlvBPDeferred = false
		a.onBreakpointsChanged()
	}
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
	// Drop unused frame-nav suppress tokens once Delve is idle again.
	if promptReady && a.isDLV() && a.dlvSuppressStopUI > 0 && stopped == nil {
		a.dlvSuppressStopUI = 0
	}
	// (gdb)/(dlv) prompt means the inferior is stopped (or not started). Clear
	// the running flag so Ctrl-Z suspends gdbforge instead of dumping ^Z into
	// :b io (regression when a stop line was missed and the flag stayed true).
	if promptReady && state != gdb.Running && a.State() != nil {
		a.Debug().SetInferiorRunning(false)
	}
	if state == gdb.Running {
		if a.State() != nil {
			a.Debug().SetInferiorRunning(true)
		}
	}
	if breakpointsChanged {
		a.onBreakpointsChanged()
	}
}

// handleDebuggerOutputMsg routes coalesced PTY output to the active backend parser.
func (a *DebuggerApp) handleDebuggerOutputMsg(msg events.GdbOutputMsg) {
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
func (a *DebuggerApp) handleGdbOutputMsg(msg events.GdbOutputMsg) {
	a.handleDebuggerOutputMsg(msg)
}

func isDlvRunCmd(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	switch strings.Fields(cmd)[0] {
	case "c", "continue", "n", "next", "s", "step", "stepout", "finish", "nexti", "ni", "stepi", "si", "restart", "run":
		return true
	default:
		return false
	}
}
