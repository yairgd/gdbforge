package main

import (
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

func (a *DebuggerApp) handleInsertKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyEscape {
		a.activateCodePane()
		return true
	}
	a.tab.HandleEvent(ev)
	return true
}

func (a *DebuggerApp) handleNormalKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyEscape {
		a.activateCodePane()
		return true
	}
	if a.handleCodeGlobalKey(ev) {
		return true
	}
	if isCopyKey(ev) {
		a.tab.HandleEvent(ev)
		return true
	}
	if ev.Key() == tcell.KeyRune && ev.Rune() == ':' {
		if a.completionBar != nil {
			a.completionBar.Clear()
		}
		a.SetMode(platform.ModeCommand)
		a.cmdWidget.Activate()
		return true
	}
	if ev.Key() == tcell.KeyRune && ev.Rune() == 'i' {
		a.activateGdbInsertMode()
		return true
	}
	if key, ok := platform.KeyFromEvent(ev); ok {
		cmd, ok := a.keyBindings.SearchPartial(key)
		if ok {
			cmd.Action()
			return true
		}
		if a.keyBindings.InPartial() {
			return true
		}
	} else {
		a.keyBindings.ResetPartial()
	}
	// Focused scrollable panes (e.g. Log) handle their bindings without insert mode.
	if w := a.tab.FocusedWidget(); w != nil {
		if h, ok := w.(termui.FocusKeyHandler); ok && h.HandleFocusKey(ev) {
			return true
		}
	}
	return true
}

// handleCodeGlobalKey routes Up/Down/Space/n/s to the active CodeWidget / GDB
// regardless of which pane is focused.
func (a *DebuggerApp) handleCodeGlobalKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		if cw := a.activeCodeWidget(); cw != nil {
			cw.MoveSel(-1)
			a.RequestFrame()
			return true
		}
	case tcell.KeyDown:
		if cw := a.activeCodeWidget(); cw != nil {
			cw.MoveSel(1)
			a.RequestFrame()
			return true
		}
	case tcell.KeyRune:
		switch ev.Rune() {
		case ' ':
			if cw := a.activeCodeWidget(); cw != nil {
				cw.BreakAtSel()
				a.RequestFrame()
				return true
			}
		case 'n':
			a.sendGdbExec("next")
			return true
		case 's':
			a.sendGdbExec("step")
			return true
		}
	}
	return false
}

func (a *DebuggerApp) handleCommandKey(ev *tcell.EventKey) bool {
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
		a.SetMode(platform.ModeCommand)
		return true
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		a.completionBar.Clear()
		a.SetMode(platform.ModeCommand)
		a.RequestFrame()
		return true

	case tcell.KeyEnter:
		if name := a.completionBar.Selected(); name != "" {
			a.cmdWidget.ApplyCompletion(name)
		}
		a.completionBar.Clear()
		a.SetMode(platform.ModeCommand)
		a.RequestFrame()
		return true

	case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
		a.completionBar.HandleEvent(ev)
		a.RequestFrame()
		return true

	case tcell.KeyTAB:
		// Cycle forward like Right.
		a.completionBar.HandleEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
		a.RequestFrame()
		return true

	default:
		// Printable / Backspace: leave wildmenu and edit the cmdline.
		a.completionBar.Clear()
		a.SetMode(platform.ModeCommand)
		a.cmdWidget.HandleEvent(ev)
		a.RequestFrame()
		return true
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
	if a.Mode() == platform.ModeCommand || a.Mode() == platform.ModeCompletion {
		return
	}

	if ev.Buttons()&tcell.ButtonPrimary != 0 {
		x, y := ev.Position()
		if !a.tab.IsSeparatorAt(x, y) && a.tab.FocusAt(x, y) {
			a.EnterInsertMode()
		}
	}

	if ev.Buttons()&(tcell.WheelUp|tcell.WheelDown) != 0 {
		x, y := ev.Position()
		if a.tab.FocusAt(x, y) {
			a.EnterInsertMode()
		}
	}

	a.tab.HandleEvent(ev)
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
	if app.completionBar != nil {
		app.completionBar.Clear()
	}
	switch app.Mode() {
	case platform.ModeCommand, platform.ModeCompletion:
		app.SetMode(platform.ModeNormal)
		app.cmdWidget.Deativate()
	}
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
	default:
		if a.gdbWidget != nil {
			a.gdbWidget.HandleEvent(ev)
		}
		if a.execWidget != nil {
			a.execWidget.HandleEvent(ev)
		}
	}
}
