package main

import (
	"strings"
	"sync/atomic"
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

const (
	gdbOutputFlushInterval = 16 * time.Millisecond
	gdbOutputFlushMaxBytes = 64 * 1024
)

// consoleHost is the narrow surface consoleCtl needs from the composition root.
// DebuggerApp implements it; consoleCtl must not depend on *DebuggerApp.
type consoleHost interface {
	Session() core.Session
	Backend() backend.Backend
	gdbBackend() *backend.GDBBackend
	dlvBackend() *backend.DLVBackend
	isDLV() bool
	GDBWidget() *widgets.GDBWidget
	State() *platform.AppState
	Debug() *debugstate.State
	Screen() tcell.Screen
	RequestFrame()
	Suspend()
	activateGdbInsertMode()
	sendInferior(tty *ptyx.TTY, send func())
	// Stop pipeline / peer-controller hooks.
	onGdbStopped(stop *gdb.MiStopMsg)
	onGdbFrameSync()
	// onGdbFrameSelected presents Code/Asm for a frame from =thread-selected
	// (CLI frame/f/up/down) — already on the UI thread.
	onGdbFrameSelected(fr gdb.MiFrameMsg)
	clearDebugInfoPanes()
	BreakpointsChanged()
	SelectedFrameLevel() int
	NoteStackNavGDB()
	NoteStackNavDLV(cmd string, curLevel int)
	DlvConfirming() bool
	DlvObserveUpdate(upd dlv.Update)
	DlvConfirmHost() string
	DeferDLVBPRefresh()
	TakeDeferredBP() bool
	TriggerPendingDebugInfoIfReady(promptReady bool)
	ApplyPendingFrameSync(promptReady, isError bool) bool
	SuppressStopUICount() int
	ClearSuppressStopUI()
}

// consoleCtl owns the debugger console domain: the PTY bridge, submit /
// interrupt / suspend / EOF intents, and MI / Delve update apply.
// Wired as wireConsole(..., a.console.onGdbConsoleSubmit, ...).
type consoleCtl struct {
	host      consoleHost
	cancelSub func()
	// bridgeGen identifies the active debugger console bridge. Bump before
	// canceling a subscription so a deliberate restart does not post gdb-exit.
	bridgeGen atomic.Uint64
}

// startGdbConsoleBridge coalesces debugger PTY chunks onto the UI event loop.
func (c *consoleCtl) startGdbConsoleBridge() {
	h := c.host
	if h == nil {
		return
	}
	sess := h.Session()
	if sess == nil || h.Screen() == nil {
		return
	}
	gen := c.bridgeGen.Add(1)
	ch, cancel := sess.Subscribe()
	c.cancelSub = cancel
	screen := h.Screen()
	go coalesceGdbOutput(ch, func(msg events.GdbOutputMsg) {
		_ = screen.PostEvent(tcell.NewEventInterrupt(msg))
	}, func() {
		// Ignore closes from an old bridge (e.g. Delve --tty restart).
		if c.bridgeGen.Load() != gen {
			return
		}
		_ = screen.PostEvent(tcell.NewEventInterrupt("gdb-exit"))
	})
}

// stopBridge cancels the console subscription and invalidates the current
// bridge generation, so a deliberate restart does not post gdb-exit.
func (c *consoleCtl) stopBridge() {
	c.bridgeGen.Add(1)
	if c.cancelSub != nil {
		c.cancelSub()
		c.cancelSub = nil
	}
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
	h := c.host
	if h == nil {
		return
	}
	if h.GDBWidget() == nil || h.Backend() == nil {
		return
	}
	if h.isDLV() {
		c.onDlvConsoleSubmit(raw)
		return
	}
	gb := h.gdbBackend()
	if gb == nil || gb.Client == nil {
		return
	}
	w := h.GDBWidget()
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
			h.activateGdbInsertMode()
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
			h.activateGdbInsertMode()
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
		h.NoteStackNavGDB()
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
	h := c.host
	if h == nil {
		return
	}
	db := h.dlvBackend()
	if db == nil || db.Client == nil || h.GDBWidget() == nil {
		return
	}
	w := h.GDBWidget()
	cli := db.Client

	cmd := raw
	if cmd == "" {
		cmd = w.LastHistory()
	}

	// Answer Delve [Y/n]? without treating the reply as a new CLI command.
	if h.DlvConfirming() {
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
		h.NoteStackNavDLV(cmd, h.SelectedFrameLevel())
	}
	if isDlvRunCmd(cmd) {
		if h.State() != nil {
			h.Debug().SetInferiorRunning(true)
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
	h := c.host
	if h == nil {
		return
	}
	if w := h.GDBWidget(); w != nil {
		w.ClearInput()
	}
	if h.Backend() == nil {
		return
	}
	// Interrupt must not wait on PTY-owner bookkeeping: GDB/Delve only leave
	// continue via ^C/SIGINT (typed commands sit unread until the prompt returns).
	// Confirming-interrupt policy lives on Confirm (onConfirmingInterrupt).
	running := h.State() != nil && h.Debug().InferiorRunning()
	_ = h.Backend().Interrupt(running, false)
	h.RequestFrame()
}

// onConfirmingInterrupt is Ctrl-C while a quit/y-n gate is open (Confirm machine).
func (c *consoleCtl) onConfirmingInterrupt() {
	h := c.host
	if h == nil {
		return
	}
	if w := h.GDBWidget(); w != nil {
		w.ClearInput()
	}
	if h.Backend() == nil {
		return
	}
	running := h.State() != nil && h.Debug().InferiorRunning()
	if h.isDLV() && h.DlvConfirming() {
		c.withGdbUIOwner(func() { _ = h.Backend().Interrupt(running, true) })
		h.RequestFrame()
		return
	}
	_ = h.Backend().Interrupt(running, false)
	h.RequestFrame()
}

// onGdbConsoleSuspend handles Ctrl-Z like GDB: SIGTSTP the inferior while it
// is running; otherwise suspend gdbforge (job control, shell `fg` to resume).
func (c *consoleCtl) onGdbConsoleSuspend() {
	h := c.host
	if h == nil {
		return
	}
	running := h.State() != nil && h.Debug().InferiorRunning()
	if running {
		if h.Backend() != nil && h.Backend().SupportsLiveInferiorTTY() {
			// GDB: SIGTSTP via MI pid tracking.
			c.withGdbUIOwner(func() { _ = h.Backend().SuspendInferior() })
			return
		}
		// Delve: no MI pid tracking yet — ^Z on the inferior TTY (cooked mode).
		if tty := c.inferiorTTY(); tty != nil {
			h.sendInferior(tty, func() { _ = tty.SendRaw("\x1a") })
			return
		}
	}
	h.Suspend()
}

func (c *consoleCtl) inferiorTTY() *ptyx.TTY {
	h := c.host
	if h == nil || h.Backend() == nil {
		return nil
	}
	return h.Backend().InferiorTTY()
}

func (c *consoleCtl) onGdbConsoleEOF() {
	h := c.host
	if h == nil {
		return
	}
	if h.isDLV() {
		if h.Backend() == nil {
			return
		}
		// Delve: send quit; it may ask for confirmation interactively.
		if w := h.GDBWidget(); w != nil {
			w.PushHistory("quit")
			w.EchoSubmit("quit")
			w.ClearInput()
		}
		c.withGdbUIOwner(func() { _ = h.Backend().SendLine("quit") })
		return
	}
	gb := h.gdbBackend()
	if gb == nil || gb.Client == nil {
		return
	}
	c.handleGdbQuitAction(gb.Client.RequestQuit(), "q")
}

func (c *consoleCtl) handleGdbQuitAction(act gdb.QuitAction, echoCmd string) {
	h := c.host
	if h == nil {
		return
	}
	gb := h.gdbBackend()
	if gb == nil || gb.Client == nil || h.GDBWidget() == nil {
		return
	}
	w := h.GDBWidget()
	switch act {
	case gdb.QuitShowConfirm:
		if echoCmd != "" {
			w.PushHistory(echoCmd)
			w.EchoSubmit(echoCmd)
		}
		w.BeginLiveHost(gdb.QuitConfirmLines(gb.Client.Quit.InferiorPID()), gdb.QuitConfirmHost)
		h.activateGdbInsertMode()
	case gdb.QuitReprompt:
		w.BeginLiveHost(gdb.QuitRepromptLines(), gdb.QuitConfirmHost)
		h.activateGdbInsertMode()
	default:
		w.ForceFollowTailAndScroll()
	}
	c.sendGdbQuitAction(act)
}

func (c *consoleCtl) sendGdbQuitAction(act gdb.QuitAction) {
	h := c.host
	if h == nil {
		return
	}
	gb := h.gdbBackend()
	if gb == nil || gb.Client == nil || !act.Sends() {
		return
	}
	c.withGdbUIOwner(func() { _ = gdb.ApplyQuitAction(gb.Client, act) })
}

func (c *consoleCtl) withGdbUIOwner(fn func()) {
	h := c.host
	if h == nil {
		return
	}
	if h.State() != nil {
		h.State().WithPTYOwner(platform.PTYOwnerUI, fn)
	} else {
		fn()
	}
}

func (c *consoleCtl) applyGdbMiUpdate(upd gdb.MiUpdate) {
	h := c.host
	if h == nil {
		return
	}
	gb := h.gdbBackend()
	if gb != nil && gb.Client != nil {
		gb.Client.Quit.Observe(upd)
	}
	silent := h.State() != nil && h.Debug().SuppressGdbConsole()
	confirming := gb != nil && gb.Client != nil && gb.Client.Quit.Confirming()
	if !silent && h.GDBWidget() != nil {
		includeTarget := h.State() != nil && h.Debug().GdbTargetPrint()
		h.GDBWidget().PaintMiDisplay(widgets.MiPaintUpdate{
			DisplayLines: upd.DisplayLines,
			TargetLines:  upd.TargetLines,
			PromptReady:  upd.PromptReady,
			PromptLine:   upd.PromptLine,
		}, confirming, includeTarget)
	}
	c.applyStopAndPromptSideEffects(upd.Stopped, upd.InferiorExited, upd.PromptReady, upd.State, upd.BreakpointsChanged, upd.FrameSelected)
}

func (c *consoleCtl) applyDlvUpdate(upd dlv.Update) {
	h := c.host
	if h == nil {
		return
	}
	silent := h.State() != nil && h.Debug().SuppressGdbConsole()
	h.DlvObserveUpdate(upd)
	confirming := h.DlvConfirming()
	if !silent && h.GDBWidget() != nil {
		h.GDBWidget().PaintDlvDisplay(upd.DisplayLines, upd.PromptReady, upd.PromptLine, confirming)
		if upd.ConfirmReady {
			host := upd.ConfirmHost
			if host == "" {
				host = h.DlvConfirmHost()
			}
			h.GDBWidget().BeginLiveHost(nil, host)
		}
	}
	// Defer BP list Query while Delve waits for y/n (Query would steal the answer).
	bpChanged := upd.BreakpointsChanged
	if bpChanged && confirming {
		h.DeferDLVBPRefresh()
		bpChanged = false
	}
	c.applyStopAndPromptSideEffects(upd.Stopped, upd.InferiorExited, upd.PromptReady, upd.State, bpChanged, nil)
	if upd.PromptReady && h.TakeDeferredBP() {
		h.BreakpointsChanged()
	}
}

func (c *consoleCtl) applyStopAndPromptSideEffects(
	stopped *gdb.MiStopMsg,
	inferiorExited bool,
	promptReady bool,
	state gdb.GdbState,
	breakpointsChanged bool,
	frameSelected *gdb.MiFrameMsg,
) {
	h := c.host
	if h == nil {
		return
	}
	if stopped != nil {
		h.onGdbStopped(stopped)
	}
	if inferiorExited {
		h.clearDebugInfoPanes()
	}
	h.TriggerPendingDebugInfoIfReady(promptReady)
	if h.ApplyPendingFrameSync(promptReady, state == gdb.Error) {
		// CLI frame/f/up/down include =thread-selected with the new frame in
		// the same batch as (gdb). Present that immediately — a follow-up
		// -stack-info-frame Query often left Code/Asm stale.
		if frameSelected != nil {
			h.onGdbFrameSelected(*frameSelected)
		} else {
			h.onGdbFrameSync()
		}
	}
	// Drop unused frame-nav suppress tokens once Delve is idle again.
	if promptReady && h.isDLV() && h.SuppressStopUICount() > 0 && stopped == nil {
		h.ClearSuppressStopUI()
	}
	// InferiorRunning drives Ctrl-Z and runtime Space-break (Ctrl-C + continue).
	// Prefer MI State==Running when present: a same-batch *stopped (Ctrl-C) then
	// ^running (auto-continue after break) must leave the flag true, or the next
	// Space only paints the UI and never installs the BP in GDB.
	// *stopped alone sets State=Done, so prompt/stop still clear the flag (Ctrl-Z).
	if h.State() != nil {
		switch {
		case state == gdb.Running:
			h.Debug().SetInferiorRunning(true)
		case promptReady || stopped != nil:
			h.Debug().SetInferiorRunning(false)
		}
	}
	if breakpointsChanged {
		h.BreakpointsChanged()
	}
}

// handleDebuggerOutputMsg routes coalesced PTY output to the active backend parser.
func (c *consoleCtl) handleDebuggerOutputMsg(msg events.GdbOutputMsg) {
	h := c.host
	if h == nil {
		return
	}
	if msg.Data == "" || h.Backend() == nil {
		return
	}
	ev := h.Backend().PushConsoleOutput(msg.Data)
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
