package main

import (
	"strings"
	"sync/atomic"
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugger"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
	"github.com/yairgd/gdbforge/internal/termui"
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
	GDBWidget() *widgets.GDBWidget
	State() *platform.AppState
	Debug() *debugstate.State
	Screen() tcell.Screen
	RequestFrame()
	Suspend()
	activateGdbInsertMode()
	sendInferior(tty *ptyx.TTY, send func())
	// Stop pipeline / peer-controller hooks.
	onGdbStopped(stop *debugger.StopInfo)
	onGdbFrameSync()
	// onGdbFrameSelected presents Code/Asm for a frame from =thread-selected
	// (CLI frame/f/up/down) — already on the UI thread.
	onGdbFrameSelected(fr debugger.FrameInfo)
	clearDebugInfoPanes()
	PublishBreakpointsChanged()
	SelectedFrameLevel() int
	NoteStackNavGDB()
	NoteStackNavDLV(cmd string, curLevel int)
	Confirming() bool
	DeferBPRefresh()
	TakeDeferredBP() bool
	TriggerPendingDebugInfoIfReady(promptReady bool)
	TriggerPendingStackRefreshIfReady(promptReady bool)
	ApplyPendingFrameSync(promptReady, isError bool) bool
	SuppressStopUICount() int
	ClearSuppressStopUI()
	MaybeEnableRemoteMode(cmd string)
	MaybeSwitchSerialConsoleOnContinue(cmd string)
	serialActive() bool
	serialOnState(stopped, promptReady, running bool)
	OutputWidget() *widgets.OutputWidget
	LogGdbMILines(msg events.GdbOutputMsg)
}

// consoleCtl owns the MI PTY bridge, CLI WireTTY lifecycle, and debugger update apply.
type consoleCtl struct {
	host      consoleHost
	cancelSub func()
	// bridgeGen identifies the active debugger console bridge. Bump before
	// canceling a subscription so a deliberate restart does not post gdb-exit.
	bridgeGen atomic.Uint64
	// cliWireGen invalidates CLI WireTTY OnExit after deliberate PTY teardown.
	cliWireGen atomic.Uint64
	// dlvLine accumulates Delve CLI input for run/stack side effects (Enter submits).
	dlvLine dlvLineTap
}

// dlvLineTap buffers xterm keystrokes until Enter so Delve run/stack commands
// typed in the GDB pane can arm InferiorRunning and suppressStopUI.
type dlvLineTap struct {
	buf []byte
}

func (t *dlvLineTap) reset() {
	if t == nil {
		return
	}
	t.buf = t.buf[:0]
}

func (t *dlvLineTap) feed(raw string, submit func(line string)) {
	if t == nil || raw == "" {
		return
	}
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		switch ch {
		case '\r', '\n':
			line := strings.TrimSpace(string(t.buf))
			t.buf = t.buf[:0]
			if line != "" && submit != nil {
				submit(line)
			}
		case 0x7f, 0x08: // backspace
			if len(t.buf) > 0 {
				t.buf = t.buf[:len(t.buf)-1]
			}
		case 0x03, 0x04: // Ctrl-C / Ctrl-D
			t.buf = t.buf[:0]
		default:
			t.buf = append(t.buf, ch)
		}
	}
}

// startGdbConsoleBridge coalesces debugger PTY chunks onto the UI event loop.
func (c *consoleCtl) Register(bus *platform.EventBus) {
	platform.Subscribe(bus, c.onGdbOutput)
	platform.Subscribe(bus, c.onAIReply)
}

func (c *consoleCtl) onGdbOutput(msg events.GdbOutputMsg) {
	h := c.host
	if h == nil {
		return
	}
	if msg.Err != nil && ptyx.ClosedError(msg.Err) {
		c.postDebuggerExit()
		return
	}
	h.LogGdbMILines(msg)
	if h.GDBWidget() != nil {
		c.handleDebuggerOutputMsg(msg)
	}
	// No RequestFrame: Run() already redraws after this interrupt.
}

func (c *consoleCtl) wireCLI(w *widgets.GDBWidget, tty *ptyx.TTY, onFrame func()) {
	if c == nil || w == nil || tty == nil {
		return
	}
	gen := c.cliWireGen.Add(1)
	opts := termui.WireTTYOpts{
		PostFrame: onFrame,
		OnExit: func() {
			if c.cliWireGen.Load() != gen {
				return
			}
			c.postDebuggerExit()
		},
	}
	if h := c.host; h != nil && h.Backend() != nil && h.Backend().WireCLILineTap() {
		c.dlvLine.reset()
		opts.OnSendRaw = c.onDlvTerminalSend
	}
	w.WireCLI(tty, opts)
}

func (c *consoleCtl) onDlvTerminalSend(raw string) {
	if c == nil || raw == "" {
		return
	}
	c.dlvLine.feed(raw, c.onDlvLineSubmitted)
}

// onDlvLineSubmitted mirrors the side effects of the old ConsolePane submit path
// (InferiorRunning + stack-nav suppress) for commands typed in the xterm GDB pane.
func (c *consoleCtl) onDlvLineSubmitted(cmd string) {
	h := c.host
	if h == nil || h.Backend() == nil || !h.Backend().WireCLILineTap() {
		return
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" || h.Confirming() {
		return
	}
	if backend.StackNavIsStackNavCmd(cmd) {
		h.NoteStackNavDLV(cmd, h.SelectedFrameLevel())
	}
	if backend.IsRunCmd(cmd) && h.State() != nil {
		h.Debug().SetInferiorRunning(true)
	}
}

func (c *consoleCtl) postDebuggerExit() {
	h := c.host
	if h == nil || h.Screen() == nil {
		return
	}
	_ = h.Screen().PostEvent(tcell.NewEventInterrupt("gdb-exit"))
}

func (c *consoleCtl) onAIReply(msg aiReplyMsg) {
	h := c.host
	if h == nil || h.GDBWidget() == nil {
		return
	}
	h.GDBWidget().AppendLines(msg.lines)
	h.RequestFrame()
}

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
	c.cliWireGen.Add(1)
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

func (c *consoleCtl) onGdbConsoleInterrupt() {
	h := c.host
	if h == nil {
		return
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
	if h.Backend() == nil {
		return
	}
	running := h.State() != nil && h.Debug().InferiorRunning()
	confirming := h.Backend().Confirming()
	c.withGdbUIOwner(func() { _ = h.Backend().Interrupt(running, confirming) })
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
	if h == nil || h.Backend() == nil {
		return
	}
	if cmd := h.Backend().ConsoleEOFCommand(); cmd != "" {
		c.withGdbUIOwner(func() { _ = h.Backend().SendLine(cmd) })
		return
	}
	gb := h.gdbBackend()
	if gb == nil || gb.Client == nil {
		return
	}
	c.handleGdbQuitAction(gb.Client.RequestQuit())
}

func (c *consoleCtl) handleGdbQuitAction(act gdb.QuitAction) {
	h := c.host
	if h == nil {
		return
	}
	if act == gdb.QuitShowConfirm {
		act = gdb.QuitSendQ
	}
	c.sendGdbQuitAction(act)
	if act.Sends() || act == gdb.QuitSendQ {
		h.activateGdbInsertMode()
	}
}

func (c *consoleCtl) sendGdbQuitAction(act gdb.QuitAction) {
	h := c.host
	if h == nil {
		return
	}
	gb := h.gdbBackend()
	if gb == nil || gb.Client == nil {
		return
	}
	if act == gdb.QuitShowConfirm {
		act = gdb.QuitSendQ
	}
	if !act.Sends() {
		return
	}
	c.withGdbUIOwner(func() {
		if cli := gb.Client.CLI; cli != nil {
			_ = gdb.ApplyQuitActionCLI(cli, act)
			return
		}
		_ = gdb.ApplyQuitAction(gb.Client, act)
	})
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

func (c *consoleCtl) applyConsoleUpdate(upd debugger.ConsoleUpdate) {
	h := c.host
	if h != nil && h.Backend() != nil && h.Backend().DeferBreakpointRefresh() {
		if upd.BreakpointsChanged && h.Confirming() {
			h.DeferBPRefresh()
			upd.BreakpointsChanged = false
		}
		if upd.PromptReady && h.TakeDeferredBP() {
			h.PublishBreakpointsChanged()
		}
	}
	c.applyStopAndPromptSideEffects(upd)
}

func (c *consoleCtl) applyStopAndPromptSideEffects(upd debugger.ConsoleUpdate) {
	h := c.host
	if h == nil {
		return
	}
	stopped := upd.Stopped
	inferiorExited := upd.InferiorExited
	promptReady := upd.PromptReady
	state := upd.State
	breakpointsChanged := upd.BreakpointsChanged
	frameSelected := upd.FrameSelected
	if stopped != nil {
		h.onGdbStopped(stopped)
	}
	if inferiorExited {
		h.clearDebugInfoPanes()
	}
	h.TriggerPendingDebugInfoIfReady(promptReady)
	h.TriggerPendingStackRefreshIfReady(promptReady)
	kgdb := h.Debug() != nil && h.Debug().KgdbMode()
	if !kgdb && h.ApplyPendingFrameSync(promptReady, state == debugger.StateError) {
		if frameSelected != nil {
			h.onGdbFrameSelected(*frameSelected)
		} else {
			h.onGdbFrameSync()
		}
	} else if kgdb && frameSelected != nil {
		h.onGdbFrameSelected(*frameSelected)
	}
	// Drop unused frame-nav suppress tokens once Delve is idle again.
	if promptReady && h.Backend() != nil && h.Backend().WireCLILineTap() && h.SuppressStopUICount() > 0 && stopped == nil {
		h.ClearSuppressStopUI()
	}
	// InferiorRunning drives Ctrl-Z and runtime Space-break (Ctrl-C + continue).
	// Prefer MI State==Running when present: a same-batch *stopped (Ctrl-C) then
	// ^running (auto-continue after break) must leave the flag true, or the next
	// Space only paints the UI and never installs the BP in GDB.
	// *stopped alone sets State=Done, so prompt/stop still clear the flag (Ctrl-Z).
	if h.State() != nil {
		switch {
		case state == debugger.StateRunning:
			h.Debug().SetInferiorRunning(true)
		case promptReady || stopped != nil:
			h.Debug().SetInferiorRunning(false)
		}
	}
	if breakpointsChanged {
		h.PublishBreakpointsChanged()
	}
	if h.serialActive() {
		running := state == debugger.StateRunning
		stoppedNow := stopped != nil
		h.serialOnState(stoppedNow, promptReady, running)
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
	c.applyConsoleUpdate(h.Backend().PushConsoleOutput(msg.Data))
}

// isExpectedPtyClose reports PTY reader errors that mean the debugger exited.
func isExpectedPtyClose(err error) bool {
	return ptyx.ClosedError(err)
}
