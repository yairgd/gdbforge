package main

import (
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

func (a *DebuggerApp) handleInsertKey(ev *tcell.EventKey) bool {
	// GDB console insert: pass all keys through so typing is native (Space, n,
	// etc.). Only Esc leaves insert mode; Tab runs MI -complete + wildmenu.
	if a.focusedIsGdb() {
		if key, ok := platform.KeyFromEvent(ev); ok {
			if key.Key == tcell.KeyEscape {
				a.onEscape()
				return true
			}
			if key.Key == tcell.KeyTAB {
				a.gdbTabComplete()
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
	a.toggleCodeBreakEnableOn(cw)
}

func (a *DebuggerApp) handleCommandKey(ev *tcell.EventKey) bool {
	// Cmdline owns completion — never keep a prior GDB wildmenu session.
	a.completionForGDB = false
	a.cmdWidget.HandleEvent(ev)
	if ev.Key() == tcell.KeyTAB && a.completionActive() {
		a.completionForGDB = false
		a.SetMode(platform.ModeCompletion)
		a.RequestFrame()
		return true
	}
	if ev.Key() == tcell.KeyEnter {
		a.clearCompletion()
		a.cmdWidget.Deativate()
		if a.Mode() == platform.ModeCommand {
			a.SetMode(platform.ModeNormal)
		}
	}
	return true
}

// handleCompletionKey owns keys while the wildmenu is open (ModeCompletion).
// Tab/arrows only move selection. Letters/backspace edit the source line
// (GDB console or cmdline) and re-query completions into the menu — no local filter.
func (a *DebuggerApp) handleCompletionKey(ev *tcell.EventKey) bool {
	if a.completionMenu == nil {
		a.leaveCompletionMode()
		return true
	}
	if a.tryKeyBindings(a.completionKeys, ev) {
		return true
	}
	isType := ev.Key() == tcell.KeyRune && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0
	isBS := ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2
	if isType || isBS {
		// Prefer cmdline when it is active so a stuck completionForGDB flag
		// cannot route :b editing into MI -complete.
		useGDB := a.completionForGDB && a.gdbWidget != nil &&
			(a.cmdWidget == nil || !a.cmdWidget.Active())
		if useGDB {
			if isType {
				a.gdbWidget.InsertInputRune(ev.Rune())
			} else {
				a.gdbWidget.BackspaceInput()
			}
			a.refreshGDBCompletionMenu()
			a.RequestFrame()
			return true
		}
		if a.cmdWidget != nil {
			a.completionForGDB = false
			a.cmdWidget.HandleEvent(ev)
			if !a.cmdWidget.Active() {
				a.clearCompletion()
				a.SetMode(platform.ModeNormal)
				a.RequestFrame()
				return true
			}
			a.refreshCmdCompletionMenu()
			a.RequestFrame()
			return true
		}
	}
	// Other keys: leave wildmenu and continue editing.
	a.clearCompletion()
	if a.completionForGDB {
		a.completionForGDB = false
		a.SetMode(platform.ModeInsert)
		a.tab.HandleEvent(ev)
	} else {
		a.SetMode(platform.ModeCommand)
		a.cmdWidget.HandleEvent(ev)
	}
	a.RequestFrame()
	return true
}

func (a *DebuggerApp) leaveCompletionMode() {
	a.clearCompletion()
	if a.completionForGDB {
		a.completionForGDB = false
		a.SetMode(platform.ModeInsert)
		return
	}
	a.completionForGDB = false
	a.SetMode(platform.ModeCommand)
}

// gdbTabComplete runs MI -complete for the GDB input line and feeds the same
// CompletionMsg / wildmenu path used by cmdline trie Completer.
func (a *DebuggerApp) gdbTabComplete() {
	if a.gdbWidget == nil {
		return
	}
	text := a.gdbWidget.InputText()
	res := gdb.Complete(a.GDB(), a.State(), text)

	// Expand to GDB's longest common prefix when it grows the line.
	if res.Completion != "" && res.Completion != text {
		a.gdbWidget.ApplyCompletion(res.Completion)
		text = res.Completion
	}

	names := res.Matches
	if len(names) == 0 && res.Completion != "" {
		names = []string{res.Completion}
	}
	a.publishGDBCompletionMenu(text, names)

	switch len(names) {
	case 0:
		// nothing
	case 1:
		// Unique match — no further completions for this word; add a trailing space.
		a.gdbWidget.ApplyCompletion(gdb.WithCompletionSpace(names[0]))
		a.clearCompletion()
	default:
		a.completionForGDB = true
		a.SetMode(platform.ModeCompletion)
	}
	a.RequestFrame()
}

// refreshGDBCompletionMenu re-runs -complete for the current GDB input and
// replaces the wildmenu. Does not apply LCP or unique matches (typing owns the
// line). Tab/arrows only move selection and must not call this.
//
// Stay in ModeCompletion across 0/1-match re-queries so further typing and
// backspace keep refreshing. Leaving on ≤1 made small candidate sets die as
// soon as the list narrowed (or -complete briefly returned empty).
func (a *DebuggerApp) refreshGDBCompletionMenu() {
	if a.gdbWidget == nil {
		a.leaveCompletionMode()
		return
	}
	text := a.gdbWidget.InputText()
	if strings.TrimSpace(text) == "" {
		a.leaveCompletionMode()
		return
	}
	res := gdb.Complete(a.GDB(), a.State(), text)
	names := res.Matches
	if len(names) == 0 && res.Completion != "" && res.Completion != text {
		names = []string{res.Completion}
	}
	a.publishGDBCompletionMenu(text, names)
	a.completionForGDB = true
	if a.Mode() != platform.ModeCompletion {
		a.SetMode(platform.ModeCompletion)
	}
}

func (a *DebuggerApp) publishGDBCompletionMenu(text string, names []string) {
	menu := gdb.MenuNames(text, names)
	// After file:, attach signatures from -symbol-info-functions
	// ("foo" → "foo(int, char *)"); apply still inserts bare name.
	// Skip the heavy MI query when not completing a linespec.
	if gdb.CompletingLinespec(text) {
		if sigs := gdb.FunctionSignatures(a.GDB(), a.State()); len(sigs) > 0 {
			menu = gdb.EnrichLinespecMenuNames(text, menu, sigs)
		}
	}
	if a.ctx.Bus != nil {
		platform.Publish(a.ctx.Bus, termui.CompletionMsg{
			Input: text,
			Token: text,
			Names: menu,
		})
	}
}

// refreshCmdCompletionMenu re-syncs the cmdline parser and replaces the wildmenu.
func (a *DebuggerApp) refreshCmdCompletionMenu() {
	if a.cmdWidget == nil || !a.cmdWidget.Active() {
		a.leaveCompletionMode()
		return
	}
	names := a.cmdWidget.CompletionNames()
	if a.ctx.Bus != nil {
		platform.Publish(a.ctx.Bus, termui.CompletionMsg{
			Input: a.cmdWidget.Text(),
			Token: a.cmdWidget.Text(),
			Names: names,
		})
	}
	if len(names) <= 1 {
		a.leaveCompletionMode()
		return
	}
	if a.Mode() != platform.ModeCompletion {
		a.SetMode(platform.ModeCompletion)
	}
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

	if a.Mode() == platform.ModeCommand || a.Mode() == platform.ModeCompletion {
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
			// Click/wheel outside the cmdline: leave command mode (like Esc),
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
				a.enterLuaMode(lw)
			} else {
				a.EnterInsertMode()
			}
		}
	}

	if wheel {
		if a.tab.FocusAt(x, y) {
			a.rememberCodeLeafFromFocus()
			if lw, ok := a.focusedWidget().(*widgets.LuaWidget); ok {
				a.enterLuaMode(lw)
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
	a.leaveLuaMode()
	a.completionForGDB = false
	a.clearCompletion()
	if a.tab != nil {
		a.tab.SetInsertActive(false)
	}
	if a.cmdWidget != nil && !a.cmdWidget.Active() {
		a.cmdWidget.Activate()
	}
	a.SetMode(platform.ModeCommand)
	a.RequestFrame()
}

// leaveCommandMode exits ':' / wildmenu (same as Esc).
func (a *DebuggerApp) leaveCommandMode() {
	a.completionForGDB = false
	a.clearCompletion()
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
		a.completionForGDB = false
		a.clearCompletion()
		a.SetMode(platform.ModeCommand)
		if a.cmdWidget != nil && !a.cmdWidget.Active() {
			a.cmdWidget.Activate()
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

func (app *DebuggerApp) handleUnknownCommand(_ termui.CommandEvent) bool {
	// TODO: show unknown command feedback in the UI
	return true
}

func (app *DebuggerApp) handleExitMode(_ termui.CommandEvent) bool {
	app.leaveCommandMode()
	return true
}

func (a *DebuggerApp) HandleInterrupt(ev *tcell.EventInterrupt) {
	switch data := ev.Data().(type) {
	case core.GdbOutputMsg:
		// Avoid per-line file logging during free-run floods (major TUI lag).
		if a.miLog != nil && !a.State().InferiorRunning() {
			for _, line := range strings.Split(data.Data, "\n") {
				a.miLog.Info(line)
			}
			if data.Err != nil {
				a.miLog.Error(data.Err.Error())
			}
		} else if a.miLog != nil && data.Err != nil {
			a.miLog.Error(data.Err.Error())
		}
		if a.gdbWidget != nil {
			a.handleGdbOutputMsg(data)
		}
		if a.outputWidget != nil && data.Data != "" {
			a.outputWidget.AppendPty(data.Data)
		}
		// No RequestFrame: Run() already redraws after this interrupt.
	case core.InferiorOutputMsg:
		if data.Data == "" {
			break
		}
		if a.outputWidget != nil {
			a.outputWidget.AppendInferior(data.Data)
		}
		// Legacy / standard GDB terminal: also mirror into the GDB console.
		if a.gdbWidget != nil && a.State().GdbTargetPrint() {
			a.gdbWidget.AppendTargetText(data.Data)
		}
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
		a.applyCodeStop(data.widget)
		a.RequestFrame()
	case breakpointsUIMsg:
		a.RequestFrame()
	case debugInfoUIMsg:
		// Models were updated off-thread; push to views and align Code on the UI thread.
		a.syncThreadViews()
		a.syncCallStackViews()
		a.syncCodeFromCallstack()
		a.applyCodeStop(a.activeCodeWidget())
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
