package main

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"syscall"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// withGlobalKeys runs mode-independent shortcuts (Ctrl-Z, Ctrl-C, Ctrl-D) before a mode handler.
func (a *DebuggerApp) withGlobalKeys(h termui.KeyHandler) termui.KeyHandler {
	return func(ev *tcell.EventKey) bool {
		if a.tryGlobalSuspend(ev) {
			return true
		}
		if a.tryGlobalInterrupt(ev) {
			return true
		}
		if a.tryGlobalEOF(ev) {
			return true
		}
		return h(ev)
	}
}

// tryGlobalSuspend handles Ctrl-Z in any mode/focus: SIGTSTP inferior if
// running, otherwise suspend gdbforge (same as GDB job control).
func (a *DebuggerApp) tryGlobalSuspend(ev *tcell.EventKey) bool {
	if !isCtrlZ(ev) {
		return false
	}
	a.console.onGdbConsoleSuspend()
	return true
}

// tryGlobalInterrupt handles Ctrl-C in any mode/focus.
// If a :lua worker job is running, cancel it (unblocks sleep/wait_port).
// Otherwise interrupt the debugger session (GDB/dlv PTY ^C).
func (a *DebuggerApp) tryGlobalInterrupt(ev *tcell.EventKey) bool {
	if !isCtrlC(ev) {
		return false
	}
	if a.lua.cancelJob() {
		if a.outputWidget != nil {
			a.outputWidget.AppendHostLine("cancelled (Ctrl-C)")
		}
		a.RequestFrame()
		return true
	}
	a.console.onGdbConsoleInterrupt()
	a.RequestFrame()
	return true
}

// tryGlobalEOF handles Ctrl-D in any mode/focus: same as GDB-console EOF
// (send q / quit; confirm if inferior alive).
func (a *DebuggerApp) tryGlobalEOF(ev *tcell.EventKey) bool {
	if !isCtrlD(ev) {
		return false
	}
	a.console.onGdbConsoleEOF()
	a.RequestFrame()
	return true
}

func isCtrlZ(ev *tcell.EventKey) bool {
	if ev == nil {
		return false
	}
	if ev.Key() == tcell.KeyCtrlZ {
		return true
	}
	// ASCII SUB (0x1A): some NewEventKey paths use Key(26) instead of KeyCtrlZ.
	if ev.Key() == tcell.Key(0x1a) {
		return true
	}
	if ev.Key() == tcell.KeyRune && ev.Rune() == 0x1a {
		return true
	}
	// KeyCtrlZ events also carry Rune 'z' + ModCtrl; KeyRune+ModCtrl variants too.
	if (ev.Rune() == 'z' || ev.Rune() == 'Z') && ev.Modifiers()&tcell.ModCtrl != 0 {
		return true
	}
	return false
}

func isCtrlC(ev *tcell.EventKey) bool {
	if ev == nil {
		return false
	}
	if ev.Key() == tcell.KeyCtrlC {
		return true
	}
	if ev.Key() == tcell.KeyRune && (ev.Rune() == 'c' || ev.Rune() == 'C') &&
		ev.Modifiers()&tcell.ModCtrl != 0 {
		return true
	}
	return false
}

func isCtrlD(ev *tcell.EventKey) bool {
	if ev == nil {
		return false
	}
	if ev.Key() == tcell.KeyCtrlD {
		return true
	}
	if ev.Key() == tcell.KeyRune && (ev.Rune() == 'd' || ev.Rune() == 'D') &&
		ev.Modifiers()&tcell.ModCtrl != 0 {
		return true
	}
	return false
}

func (a *DebuggerApp) handleInsertKey(ev *tcell.EventKey) bool {
	// GDB console insert: pass all keys through so typing is native (Space, n,
	// etc.). Only Esc leaves insert mode; Tab runs completion + wildmenu
	// (MI -complete for GDB, command-name list for Delve).
	if a.focusedIsGdb() {
		if key, ok := platform.KeyFromEvent(ev); ok {
			if key.Key == tcell.KeyEscape {
				a.onEscape()
				return true
			}
			if key.Key == tcell.KeyTAB {
				a.comp.gdbTabComplete()
				return true
			}
		}
		a.tab.HandleEvent(ev)
		return true
	}
	if a.tryKeyBindings(a.insertKeys, ev) {
		return true
	}
	a.tab.HandleEvent(ev)
	return true
}

func (a *DebuggerApp) handleNormalKey(ev *tcell.EventKey) bool {
	// Layout hook reserved for future layout-specific binds (currently no-op).
	if a.currentLayoutBehavior().HandleNormalKey(a, ev) {
		return true
	}
	if a.tryKeyBindings(a.keyBindings, ev) {
		return true
	}
	if isCopyKey(ev) {
		a.tab.HandleEvent(ev)
		return true
	}
	// Focused scrollable panes (e.g. Log) handle their bindings without insert mode.
	if w := a.focusedWidget(); w != nil {
		if h, ok := w.(termui.FocusKeyHandler); ok && h.HandleFocusKey(ev) {
			return true
		}
	}
	return true
}

// toggleCodeBreakEnable toggles enable/disable at the active CodeWidget cursor
// (same as BreakpointWidget e). Disabled marks stay in the BP list and show yellow.
func (a *DebuggerApp) toggleCodeBreakEnable() {
	cw := a.activeCodeWidget()
	if focused := a.focusedCode(); focused != nil {
		cw = focused
	}
	a.breaks.toggleCodeBreakEnableOn(cw)
}

func (a *DebuggerApp) handleCommandKey(ev *tcell.EventKey) bool {
	// Cmdline owns completion — never keep a prior GDB wildmenu session.
	a.comp.setForGDB(false)
	a.cmdWidget.HandleEvent(ev)
	if ev.Key() == tcell.KeyTAB && a.comp.active() {
		a.comp.setForGDB(false)
		a.SetMode(platform.ModeCompletion)
		a.RequestFrame()
		return true
	}
	if ev.Key() == tcell.KeyEnter {
		a.comp.clear()
		a.cmdWidget.Deativate()
		if a.Mode() == platform.ModeCommand {
			a.SetMode(platform.ModeNormal)
		}
	}
	return true
}

// handleSearchKey owns keys while ModeSearch ('/' cmdline) is active.
// Same edit line as command mode, but no tab-completion / command parse —
// edits live-preview matches on the search target; Enter commits.
func (a *DebuggerApp) handleSearchKey(ev *tcell.EventKey) bool {
	a.comp.setForGDB(false)
	a.comp.clear()
	a.cmdWidget.HandleEvent(ev)
	if !a.cmdWidget.Active() && a.Mode() == platform.ModeSearch {
		a.SetMode(platform.ModeNormal)
	}
	a.RequestFrame()
	return true
}

// trySearchOrGdbNext is normal-mode n: on Code always GDB next (like s/c);
// elsewhere search-next when a pattern is active, else GDB next.
func (a *DebuggerApp) trySearchOrGdbNext() bool {
	if a.focusedIsCode() {
		a.sendGdbExec("next")
		return true
	}
	if a.search.hasActivePattern() {
		a.search.nextMatch()
		return true
	}
	a.sendGdbExec("next")
	return true
}

// handleCompletionKey owns keys while the wildmenu is open (ModeCompletion).
// Tab/arrows only move selection. Letters/backspace edit the source line
// (GDB console or cmdline) and re-query completions into the menu — no local filter.
func (a *DebuggerApp) handleCompletionKey(ev *tcell.EventKey) bool {
	if !a.comp.hasMenu() {
		a.comp.leaveMode()
		return true
	}
	if a.tryKeyBindings(a.completionKeys, ev) {
		return true
	}
	isType := ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0
	isBS := ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2
	if isType || isBS {
		if a.comp.useGDBInput() {
			if isType {
				a.gdbWidget.InsertInputRune(ev.Rune())
			} else {
				a.gdbWidget.BackspaceInput()
			}
			a.comp.refreshGDBMenu()
			a.RequestFrame()
			return true
		}
		if a.cmdWidget != nil {
			a.comp.setForGDB(false)
			a.cmdWidget.HandleEvent(ev)
			if !a.cmdWidget.Active() {
				a.comp.clear()
				a.SetMode(platform.ModeNormal)
				a.RequestFrame()
				return true
			}
			a.comp.refreshCmdMenu()
			a.RequestFrame()
			return true
		}
	}
	// Other keys: leave wildmenu and continue editing.
	a.comp.clear()
	if a.comp.isForGDB() {
		a.comp.setForGDB(false)
		a.SetMode(platform.ModeInsert)
		a.tab.HandleEvent(ev)
	} else {
		a.SetMode(platform.ModeCommand)
		a.cmdWidget.HandleEvent(ev)
	}
	a.RequestFrame()
	return true
}

func isCopyKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyCtrlC || ev.Key() == tcell.KeyCtrlX || ev.Key() == tcell.KeyCtrlV {
		return true
	}
	if ev.Modifiers()&tcell.ModCtrl == 0 || ev.Key() != tcell.KeyRune {
		return false
	}
	switch ev.Rune() {
	case 'c', 'C', 'x', 'X', 'v', 'V':
		return true
	}
	return false
}

func (a *DebuggerApp) HandleMouse(ev *tcell.EventMouse) {
	x, y := ev.Position()
	primary := ev.Buttons()&tcell.ButtonPrimary != 0
	wheel := ev.Buttons()&(tcell.WheelUp|tcell.WheelDown) != 0
	inCmd := a.cmdLineContains(x, y)

	if a.Mode() == platform.ModeCommand || a.Mode() == platform.ModeSearch || a.Mode() == platform.ModeCompletion {
		// Middle-click paste into the cmdline (Linux terminal convention).
		if a.cmdWidget != nil && ev.Buttons()&tcell.ButtonMiddle != 0 {
			a.cmdWidget.HandleEvent(ev)
			a.RequestFrame()
			return
		}
		if primary && inCmd {
			a.clickCmdLine(x)
			return
		}
		if (primary || wheel) && !inCmd {
			// Click/wheel outside the cmdline: leave command/search mode (like Esc),
			// then fall through so the pane under the pointer can take focus.
			a.leaveCommandMode()
		} else {
			return
		}
	}

	if primary && inCmd {
		a.enterCommandMode()
		a.clickCmdLine(x)
		return
	}

	if primary {
		if !a.tab.IsSeparatorAt(x, y) && a.tab.FocusAt(x, y) {
			a.rememberCodeLeafFromFocus()
			if lw, ok := a.focusedWidget().(*widgets.LuaWidget); ok {
				a.lua.enterMode(lw)
			} else {
				a.EnterInsertMode()
			}
		}
	}

	if wheel {
		if a.tab.FocusAt(x, y) {
			a.rememberCodeLeafFromFocus()
			if lw, ok := a.focusedWidget().(*widgets.LuaWidget); ok {
				a.lua.enterMode(lw)
			} else {
				a.EnterInsertMode()
			}
		}
	}

	a.tab.HandleEvent(ev)
	// Always repaint so green stop marks / selection update after clicks.
	a.RequestFrame()
}

// enterCommandMode activates the ':' cmdline (same as pressing ':').
// Leaves insert-active so the focused pane status is blue, matching Esc then ':'.
func (a *DebuggerApp) enterCommandMode() {
	a.lua.leaveMode()
	a.comp.setForGDB(false)
	a.comp.clear()
	if a.tab != nil {
		a.tab.SetInsertActive(false)
	}
	a.search.clearTarget()
	if a.cmdWidget != nil && !a.cmdWidget.Active() {
		a.cmdWidget.Activate()
	}
	a.SetMode(platform.ModeCommand)
	a.RequestFrame()
}

// enterSearchMode activates the '/' cmdline (same as pressing '/').
// Captures the last active (focused) pane — green/blue status — as search target.
func (a *DebuggerApp) enterSearchMode() {
	a.lua.leaveMode()
	a.comp.setForGDB(false)
	a.comp.clear()
	if a.tab != nil {
		a.tab.SetInsertActive(false)
	}
	a.search.captureFocused()
	if a.cmdWidget != nil {
		a.cmdWidget.ActivateSearch()
	}
	a.SetMode(platform.ModeSearch)
	a.RequestFrame()
}

// leaveCommandMode exits ':' / '/' / wildmenu (same as Esc).
func (a *DebuggerApp) leaveCommandMode() {
	wasSearch := a.Mode() == platform.ModeSearch ||
		(a.cmdWidget != nil && a.cmdWidget.Kind() == termui.CmdKindSearch)
	a.comp.setForGDB(false)
	a.comp.clear()
	if wasSearch {
		a.search.revertPreview()
		// Keep search target so n/N and */# still work on the committed pattern.
	}
	if a.cmdWidget != nil {
		a.cmdWidget.Deativate()
	}
	a.SetMode(platform.ModeNormal)
	a.RequestFrame()
}

func (a *DebuggerApp) cmdLineContains(x, y int) bool {
	if a.cmdWidget == nil {
		return false
	}
	for _, n := range a.Widgets() {
		if n.Widget() == a.cmdWidget {
			return n.Rect().Contains(x, y)
		}
	}
	return false
}

func (a *DebuggerApp) clickCmdLine(screenX int) {
	if a.cmdWidget == nil {
		return
	}
	originX := 0
	for _, n := range a.Widgets() {
		if n.Widget() == a.cmdWidget {
			originX = n.Rect().X()
			break
		}
	}
	if a.Mode() == platform.ModeCompletion {
		a.comp.setForGDB(false)
		a.comp.clear()
		if a.cmdWidget != nil && a.cmdWidget.Kind() == termui.CmdKindSearch {
			a.SetMode(platform.ModeSearch)
			if !a.cmdWidget.Active() {
				a.cmdWidget.ActivateSearch()
			}
		} else {
			a.SetMode(platform.ModeCommand)
			if a.cmdWidget != nil && !a.cmdWidget.Active() {
				a.cmdWidget.Activate()
			}
		}
	}
	a.cmdWidget.SetCursorAtLocalX(screenX - originX)
	a.RequestFrame()
}

func (a *DebuggerApp) HandleResize() {
	c := a.UpdateCanvas()

	w := a.Widgets()
	if len(w) < 3 {
		return
	}
	// Tab keeps full height (it insets H-2 internally). Bar overlays H-2; cmd at H-1.
	w[0].SetRect(c.ChildRect(0, 0, c.W(), c.H()))
	w[1].SetRect(c.ChildRect(0, c.H()-2, c.W(), 1))
	w[2].SetRect(c.ChildRect(0, c.H()-1, c.W(), 1))
}

func (app *DebuggerApp) handleUnknownCommand(ev termui.CommandEvent) bool {
	if msg, ok := ev.(termui.SubmitMsg); ok && app.tryGotoLineCmd(msg.Text) {
		return true
	}
	// TODO: show unknown command feedback in the UI
	return true
}

// tryGotoLineCmd handles Vim-style :N / :0 — jump browse cursor to line N
// in the active Code buffer (blue line). :0 goes to line 1.
func (a *DebuggerApp) tryGotoLineCmd(text string) bool {
	line, ok := parseGotoLineCmd(text)
	if !ok {
		return false
	}
	cw := a.activeCodeWidget()
	if focused := a.focusedCode(); focused != nil {
		cw = focused
	}
	if cw == nil {
		return true // consumed as goto, but no buffer
	}
	cw.GotoLine(line)
	a.RequestFrame()
	return true
}

// parseGotoLineCmd accepts ":42", "42", ":0" (→ line 1). Rejects non-numeric.
func parseGotoLineCmd(text string) (line int, ok bool) {
	s := strings.TrimSpace(text)
	if strings.HasPrefix(s, ":") {
		s = strings.TrimSpace(s[1:])
	}
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	if n < 1 {
		n = 1
	}
	return n, true
}

func (app *DebuggerApp) handleExitMode(_ termui.CommandEvent) bool {
	app.leaveCommandMode()
	return true
}

func (a *DebuggerApp) HandleInterrupt(ev *tcell.EventInterrupt) {
	switch data := ev.Data().(type) {
	case events.GdbOutputMsg:
		// Avoid per-line file logging during free-run floods (major TUI lag).
		if a.miLog != nil && !a.Debug().InferiorRunning() {
			for _, line := range strings.Split(data.Data, "\n") {
				a.miLog.Info(line)
			}
			if data.Err != nil && !isExpectedPtyClose(data.Err) {
				a.miLog.Error(data.Err.Error())
			}
		} else if a.miLog != nil && data.Err != nil && !isExpectedPtyClose(data.Err) {
			a.miLog.Error(data.Err.Error())
		}
		if a.gdbWidget != nil {
			a.console.handleDebuggerOutputMsg(data)
		}
		if a.outputWidget != nil && data.Data != "" {
			a.outputWidget.AppendPty(data.Data)
		}
		// No RequestFrame: Run() already redraws after this interrupt.
	case events.InferiorOutputMsg:
		if data.Data == "" {
			break
		}
		if a.outputWidget != nil {
			a.outputWidget.AppendInferior(data.Data)
		}
		// Mirror into the debugger console when enabled (default on for Delve).
		if a.gdbWidget != nil && a.Debug().GdbTargetPrint() {
			a.gdbWidget.AppendTargetText(data.Data)
		}
		a.RequestFrame()
	case core.ExecOutputMsg:
		if a.execWidget != nil {
			a.execWidget.HandleEvent(ev)
		}
	case aiReplyMsg:
		if a.gdbWidget != nil {
			a.gdbWidget.AppendLines(data.lines)
			a.RequestFrame()
		}
	case codeRefreshMsg:
		if data.fromStop {
			// Late stop paint lost the race with call-stack / frame browse.
			if data.stopGen != a.dlv.codeNavGen {
				a.RequestFrame()
				break
			}
			a.debugInfo.selectLevel(0)
			if data.stop != nil {
				_ = a.updateCodeAfterStop(data.stop)
				// Repaint all Code gutters from the model (fresh from the
				// pre-stop -break-list query, or whatever Merge already holds).
				if a.breaks.List() != nil {
					a.breaks.paintCodeMarks(a.breaks.Items())
				}
				a.RequestFrame()
				break
			}
		}
		// Console frame/f/up/down: present with the fetched frame (not nil).
		if data.frame != nil {
			a.debugInfo.syncCallStackViews()
			a.debugInfo.selectLevel(data.frame.Level)
			a.showFrameSource(*data.frame)
			a.RequestFrame()
			break
		}
		if data.widget != nil {
			a.presentLocation(data.widget, nil)
		}
		a.RequestFrame()
	case asmRefreshMsg:
		a.asm.applyRefresh(data)
		a.RequestFrame()
	case breakpointsUIMsg:
		// refreshBreakpoints may have applied off-thread; push gutters again
		// on the UI thread so a late Code buffer still gets marks.
		if a.breaks.List() != nil {
			a.breaks.syncBreakpointViews()
		}
		a.RequestFrame()
	case debugInfoUIMsg:
		// Models were updated off-thread; push to views on the UI thread.
		// Do not force frame 0 or re-drive Code here — that races with call-stack browse.
		a.debugInfo.syncThreadViews()
		a.debugInfo.syncCallStackViews()
		a.syncCodeFromCallstack()
		a.RequestFrame()
	case luaUIMsg:
		func() {
			defer func() {
				if data.done != nil {
					close(data.done)
				}
			}()
			if data.fn != nil {
				data.fn()
			}
		}()
	case luaJobDoneMsg:
		if data.err != nil {
			msg := data.err.Error()
			// Ctrl-C already printed "cancelled (Ctrl-C)"; skip duplicate.
			if !errors.Is(data.err, luahost.ErrJobCancelled) &&
				!strings.Contains(msg, "cancelled") &&
				!strings.Contains(msg, "context canceled") {
				if a.outputWidget != nil {
					a.outputWidget.AppendHostLine(data.name + ": " + msg)
				}
				if a.ctx.Log != nil {
					a.ctx.Log.Named("lua").Error(data.name + ": " + msg)
				}
			}
		}
		a.RequestFrame()
	case string:
		// GDB PTY closed (q / quit / -gdb-exit) — leave the app.
		if data == "gdb-exit" {
			a.Exit()
			return
		}
		if a.gdbWidget != nil {
			a.gdbWidget.HandleEvent(ev)
		}
		if a.execWidget != nil {
			a.execWidget.HandleEvent(ev)
		}
	default:
		if a.gdbWidget != nil {
			a.gdbWidget.HandleEvent(ev)
		}
		if a.execWidget != nil {
			a.execWidget.HandleEvent(ev)
		}
	}
}

// isExpectedPtyClose reports PTY reader errors that mean the debugger exited
// (q / quit / process death). Linux often returns EIO ("input/output error")
// on /dev/ptmx once the slave closes — not a startup or session-setup failure.
func isExpectedPtyClose(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == syscall.EIO {
		return true
	}
	// Wrapped "read /dev/ptmx: input/output error" from older readers.
	msg := err.Error()
	return strings.Contains(msg, "input/output error") ||
		strings.Contains(msg, "file already closed")
}
