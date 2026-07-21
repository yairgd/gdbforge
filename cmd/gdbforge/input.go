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

func (a *DebuggerApp) toggleCodeBreakEnableOn(cw *widgets.CodeWidget) {
	if cw == nil || a.bpWidget == nil {
		return
	}
	path := cw.Path()
	line := cw.SelLine()
	if path == "" || line < 1 {
		return
	}
	// Toggle when listed, or when CodeWidget still shows an enabled mark not yet merged.
	if a.bpWidget.ToggleAtFileLine(path, line, cw.HasEnabledBreak(line)) {
		a.RequestFrame()
	}
}

func (a *DebuggerApp) handleCommandKey(ev *tcell.EventKey) bool {
	a.completionForGDB = false
	a.cmdWidget.HandleEvent(ev)
	if ev.Key() == tcell.KeyTAB && a.completionBar != nil && a.completionBar.Active() {
		a.SetMode(platform.ModeCompletion)
		a.RequestFrame()
		return true
	}
	if ev.Key() == tcell.KeyEnter {
		if a.completionBar != nil {
			a.completionBar.Clear()
		}
		a.cmdWidget.Deativate()
		if a.Mode() == platform.ModeCommand {
			a.SetMode(platform.ModeNormal)
		}
	}
	return true
}

// handleCompletionKey owns keys while the wildmenu is open (ModeCompletion).
func (a *DebuggerApp) handleCompletionKey(ev *tcell.EventKey) bool {
	if a.completionBar == nil {
		a.leaveCompletionMode()
		return true
	}
	if a.tryKeyBindings(a.completionKeys, ev) {
		return true
	}
	// Printable / Backspace: leave wildmenu and continue editing.
	a.completionBar.Clear()
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
	if a.completionBar != nil {
		a.completionBar.Clear()
	}
	if a.completionForGDB {
		a.completionForGDB = false
		a.SetMode(platform.ModeInsert)
		return
	}
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
	// Wildmenu: only the word being completed, not prior words.
	menu := gdb.MenuNames(text, names)

	if a.ctx.Bus != nil {
		platform.Publish(a.ctx.Bus, termui.CompletionMsg{
			Input: text,
			Token: text,
			Names: menu,
		})
	}

	switch len(names) {
	case 0:
		// nothing
	case 1:
		a.gdbWidget.ApplyCompletion(names[0])
		if a.completionBar != nil {
			a.completionBar.Clear()
		}
	default:
		a.completionForGDB = true
		a.SetMode(platform.ModeCompletion)
	}
	a.RequestFrame()
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
		if primary && !inCmd {
			// Click outside the cmdline: leave command mode (like Esc), then
			// fall through so the pane under the pointer can take focus.
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
			a.EnterInsertMode()
		}
	}

	if ev.Buttons()&(tcell.WheelUp|tcell.WheelDown) != 0 {
		if a.tab.FocusAt(x, y) {
			a.rememberCodeLeafFromFocus()
			a.EnterInsertMode()
		}
	}

	a.tab.HandleEvent(ev)
}

// enterCommandMode activates the ':' cmdline (same as pressing ':').
// Leaves insert-active so the focused pane status is blue, matching Esc then ':'.
func (a *DebuggerApp) enterCommandMode() {
	if a.completionBar != nil {
		a.completionBar.Clear()
	}
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
	if a.completionBar != nil {
		a.completionBar.Clear()
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
		if a.completionBar != nil {
			a.completionBar.Clear()
		}
		a.SetMode(platform.ModeCommand)
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
			a.gdbWidget.HandleEvent(ev)
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
		a.RequestFrame()
	case string:
		// GDB PTY closed (q / quit / -gdb-exit) — leave the app like Ctrl-D.
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
