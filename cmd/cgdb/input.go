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
		a.tab.SetInsertActive(false)
		a.SetMode(platform.ModeNormal)
		a.RequestRedraw()
		return true
	}
	a.tab.HandleEvent(ev)
	return true
}

func (a *DebuggerApp) handleNormalKey(ev *tcell.EventKey) bool {
	if isCopyKey(ev) {
		a.tab.HandleEvent(ev)
		return true
	}
	if ev.Key() == tcell.KeyRune && ev.Rune() == ':' {
		a.SetMode(platform.ModeCommand)
		a.cmdWidget.Activate()
		return true
	}
	if ev.Key() == tcell.KeyRune && ev.Rune() == 'i' {
		a.EnterInsertMode()
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

func (a *DebuggerApp) handleCommandKey(ev *tcell.EventKey) bool {
	a.cmdWidget.HandleEvent(ev)
	if ev.Key() == tcell.KeyEnter {
		a.cmdWidget.Deativate()
		if a.Mode() == platform.ModeCommand {
			a.SetMode(platform.ModeNormal)
		}
	}
	return true
}

func isCopyKey(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyCtrlC ||
		(ev.Key() == tcell.KeyRune && ev.Rune() == 'c' && ev.Modifiers()&tcell.ModCtrl != 0)
}

func (a *DebuggerApp) HandleMouse(ev *tcell.EventMouse) {
	if a.Mode() == platform.ModeCommand {
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
	if len(w) < 2 {
		return
	}
	w[0].SetRect(c.ChildRect(0, 0, c.W(), c.H()))
	w[1].SetRect(c.ChildRect(0, c.H()-1, c.W(), 1))
}

func (app *DebuggerApp) handleUnknownCommand(_ termui.CommandEvent) bool {
	// TODO: show unknown command feedback in the UI
	return true
}

func (app *DebuggerApp) handleExitMode(_ termui.CommandEvent) bool {
	if app.Mode() == platform.ModeCommand {
		app.SetMode(platform.ModeNormal)
		app.cmdWidget.Deativate()
	}
	return true
}

func (a *DebuggerApp) HandleInterrupt(ev *tcell.EventInterrupt) {
	if msg, ok := ev.Data().(core.GdbOutputMsg); ok && a.miLog != nil {
		// Log every raw MI chunk line for the top Log pane (and file sink).
		for _, line := range strings.Split(msg.Data, "\n") {
			a.miLog.Info(line)
		}
		if msg.Err != nil {
			a.miLog.Error(msg.Err.Error())
		}
	}
	if a.gdbWidget != nil {
		a.gdbWidget.HandleEvent(ev)
	}
}
