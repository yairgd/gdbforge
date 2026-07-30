package main

import (
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
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
func (c *consoleCtl) startGdbConsoleBridge() {
	a := c.app
	if a == nil {
		return
	}
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
	coalescePtyOutput(ch, ptyCoalesceOpts{
		Interval: gdbOutputFlushInterval,
		MaxBytes: gdbOutputFlushMaxBytes,
		Post: func(data string, err error) {
			if post == nil {
				return
			}
			post(events.GdbOutputMsg{Data: data, Err: err})
		},
		OnExit: onExit,
	})
}

func (c *consoleCtl) onGdbConsoleSubmit(raw string) {
	a := c.app
	if a == nil {
		return
	}
	if a.gdbWidget == nil || a.backend == nil {
		return
	}
	if a.isDLV() {
		c.onDlvConsoleSubmit(raw)
		return
	}
	gb := a.gdbBackend()
	if gb == nil || gb.Client == nil {
		return
	}
	w := a.gdbWidget
	cli := gb.Client

	cmd := raw
	if !cli.Quit.Confirming() && cmd == "" {
		cmd = w.LastHistory()
	}

	if cli.Quit.Confirming() {
		ans := strings.TrimSpace(strings.ToLower(raw))
		display := ans
		if display == "" {
			display = "n"
		}
		act := cli.Quit.Answer(raw)
		if act == gdb.QuitReprompt {
			w.BeginLiveHost(gdb.QuitRepromptLines(), gdb.QuitConfirmHost)
			return
		}
		w.EchoSubmit(display)
		w.ClearInput()
		c.sendGdbQuitAction(act)
		w.ForceFollowTailAndScroll()
		return
	}

	if act := cli.Quit.SubmitQuitCommand(cmd); act != gdb.QuitNoop {
		if act == gdb.QuitShowConfirm {
			if cmd != "" {
				w.PushHistory(cmd)
				w.EchoSubmit(cmd)
			}
			w.BeginLiveHost(gdb.QuitConfirmLines(cli.Quit.InferiorPID()), gdb.QuitConfirmHost)
			return
		}
		if cmd != "" {
			w.PushHistory(cmd)
			w.EchoSubmit(cmd)
		}
		w.ClearInput()
		c.sendGdbQuitAction(act)
		w.ForceFollowTailAndScroll()
		return
	}

	if gdb.IsStackNavCmd(cmd) {
		a.codeNavGen++
		a.pendingFrameSync = true
	}
	// Run-control via MI so the GDB pane does not dump CLI source/line listings
	// (Code widget already follows *stopped).
	sendCmd := gdb.CLIExecToMI(cmd)
	send := func() { _ = cli.Send(sendCmd) }
	if cmd != "" {
		w.PushHistory(cmd)
		w.EchoSubmit(cmd)
	}
	c.withGdbUIOwner(send)
	w.ClearInput()
	w.ForceFollowTailAndScroll()
}

func (c *consoleCtl) onDlvConsoleSubmit(raw string) {
	a := c.app
	if a == nil {
		return
	}
	db := a.dlvBackend()
	if db == nil || db.Client == nil || a.gdbWidget == nil {
		return
	}
	w := a.gdbWidget
	cli := db.Client

	cmd := raw
	if cmd == "" {
		cmd = w.LastHistory()
	}

	// Answer Delve [Y/n]? without treating the reply as a new CLI command.
	if a.dlvConfirm.Confirming() {
		send := func() { _ = cli.Send(cmd) }
		if cmd != "" {
			w.EchoSubmit(cmd)
		}
		c.withGdbUIOwner(send)
		w.ClearInput()
		w.ForceFollowTailAndScroll()
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
	send := func() { _ = cli.Send(cmd) }
	if cmd != "" {
		w.PushHistory(cmd)
		w.EchoSubmit(cmd)
	}
	c.withGdbUIOwner(send)
	w.ClearInput()
	w.ForceFollowTailAndScroll()
}

func (c *consoleCtl) onGdbConsoleInterrupt() {
	a := c.app
	if a == nil {
		return
	}
	if a.gdbWidget != nil {
		a.gdbWidget.ClearInput()
	}
	if a.backend == nil {
		return
	}
	// Interrupt must not wait on PTY-owner bookkeeping: GDB/Delve only leave
	// continue via ^C/SIGINT (typed commands sit unread until the prompt returns).
	running := a.State() != nil && a.Debug().InferiorRunning()
	confirming := a.dlvConfirm.Confirming()
	if confirming && a.isDLV() {
		c.withGdbUIOwner(func() { _ = a.backend.Interrupt(running, true) })
		a.RequestFrame()
		return
	}
	_ = a.backend.Interrupt(running, false)
	a.RequestFrame()
}

// onGdbConsoleSuspend handles Ctrl-Z like GDB: SIGTSTP the inferior while it
// is running; otherwise suspend gdbforge (job control, shell `fg` to resume).
func (c *consoleCtl) onGdbConsoleSuspend() {
	a := c.app
	if a == nil {
		return
	}
	running := a.State() != nil && a.Debug().InferiorRunning()
	if running {
		if a.backend != nil && a.backend.SupportsLiveInferiorTTY() {
			// GDB: SIGTSTP via MI pid tracking.
			c.withGdbUIOwner(func() { _ = a.backend.SuspendInferior() })
			return
		}
		// Delve: no MI pid tracking yet — ^Z on the inferior TTY (cooked mode).
		if tty := c.inferiorTTY(); tty != nil {
			a.sendInferior(tty, func() { _ = tty.SendRaw("\x1a") })
			return
		}
	}
	a.Suspend()
}

func (c *consoleCtl) inferiorTTY() *ptyx.TTY {
	a := c.app
	if a == nil || a.backend == nil {
		return nil
	}
	return a.backend.InferiorTTY()
}

func (c *consoleCtl) onGdbConsoleEOF() {
	a := c.app
	if a == nil {
		return
	}
	if a.isDLV() {
		if a.backend == nil {
			return
		}
		// Delve: send quit; it may ask for confirmation interactively.
		w := a.gdbWidget
		if w != nil {
			w.PushHistory("quit")
			w.EchoSubmit("quit")
			w.ClearInput()
		}
		c.withGdbUIOwner(func() { _ = a.backend.SendLine("quit") })
		return
	}
	gb := a.gdbBackend()
	if gb == nil || gb.Client == nil {
		return
	}
	c.handleGdbQuitAction(gb.Client.RequestQuit(), "q")
}

func (c *consoleCtl) handleGdbQuitAction(act gdb.QuitAction, echoCmd string) {
	a := c.app
	if a == nil {
		return
	}
	gb := a.gdbBackend()
	if gb == nil || gb.Client == nil || a.gdbWidget == nil {
		return
	}
	w := a.gdbWidget
	switch act {
	case gdb.QuitShowConfirm:
		if echoCmd != "" {
			w.PushHistory(echoCmd)
			w.EchoSubmit(echoCmd)
		}
		w.BeginLiveHost(gdb.QuitConfirmLines(gb.Client.Quit.InferiorPID()), gdb.QuitConfirmHost)
	case gdb.QuitReprompt:
		w.BeginLiveHost(gdb.QuitRepromptLines(), gdb.QuitConfirmHost)
	default:
		w.ForceFollowTailAndScroll()
	}
	c.sendGdbQuitAction(act)
}

func (c *consoleCtl) sendGdbQuitAction(act gdb.QuitAction) {
	a := c.app
	if a == nil {
		return
	}
	gb := a.gdbBackend()
	if gb == nil || gb.Client == nil || !act.Sends() {
		return
	}
	c.withGdbUIOwner(func() { _ = gdb.ApplyQuitAction(gb.Client, act) })
}

func (c *consoleCtl) withGdbUIOwner(fn func()) {
	a := c.app
	if a == nil {
		return
	}
	if a.State() != nil {
		a.State().WithPTYOwner(platform.PTYOwnerUI, fn)
	} else {
		fn()
	}
}

func (c *consoleCtl) applyGdbMiUpdate(upd gdb.MiUpdate) {
	a := c.app
	if a == nil {
		return
	}
	if gb := a.gdbBackend(); gb != nil && gb.Client != nil {
		gb.Client.Quit.Observe(upd)
	}
	silent := a.State() != nil && a.Debug().SuppressGdbConsole()
	gb := a.gdbBackend()
	confirming := gb != nil && gb.Client != nil && gb.Client.Quit.Confirming()
	if !silent && a.gdbWidget != nil {
		includeTarget := a.State() != nil && a.Debug().GdbTargetPrint()
		a.gdbWidget.PaintMiDisplay(widgets.MiPaintUpdate{
			DisplayLines: upd.DisplayLines,
			TargetLines:  upd.TargetLines,
			PromptReady:  upd.PromptReady,
			PromptLine:   upd.PromptLine,
		}, confirming, includeTarget)
	}
	c.applyStopAndPromptSideEffects(upd.Stopped, upd.InferiorExited, upd.PromptReady, upd.State, upd.BreakpointsChanged)
}

func (c *consoleCtl) applyDlvUpdate(upd dlv.Update) {
	a := c.app
	if a == nil {
		return
	}
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
	c.applyStopAndPromptSideEffects(upd.Stopped, upd.InferiorExited, upd.PromptReady, upd.State, bpChanged)
	if upd.PromptReady && a.dlvBPDeferred {
		a.dlvBPDeferred = false
		a.onBreakpointsChanged()
	}
}

func (c *consoleCtl) applyStopAndPromptSideEffects(
	stopped *gdb.MiStopMsg,
	inferiorExited bool,
	promptReady bool,
	state gdb.GdbState,
	breakpointsChanged bool,
) {
	a := c.app
	if a == nil {
		return
	}
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
	// InferiorRunning drives Ctrl-Z and runtime Space-break (Ctrl-C + continue).
	// Prefer MI State==Running when present: a same-batch *stopped (Ctrl-C) then
	// ^running (auto-continue after break) must leave the flag true, or the next
	// Space only paints the UI and never installs the BP in GDB.
	// *stopped alone sets State=Done, so prompt/stop still clear the flag (Ctrl-Z).
	if a.State() != nil {
		switch {
		case state == gdb.Running:
			a.Debug().SetInferiorRunning(true)
		case promptReady || stopped != nil:
			a.Debug().SetInferiorRunning(false)
		}
	}
	if breakpointsChanged {
		a.onBreakpointsChanged()
	}
}

// handleDebuggerOutputMsg routes coalesced PTY output to the active backend parser.
func (c *consoleCtl) handleDebuggerOutputMsg(msg events.GdbOutputMsg) {
	a := c.app
	if a == nil {
		return
	}
	if msg.Data == "" || a.backend == nil {
		return
	}
	ev := a.backend.PushConsoleOutput(msg.Data)
	if ev.GDB != nil {
		c.applyGdbMiUpdate(*ev.GDB)
		return
	}
	if u := backend.AsDLVUpdate(ev); u != nil {
		c.applyDlvUpdate(*u)
	}
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
