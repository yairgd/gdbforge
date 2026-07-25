package main

import (
	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

func (a *DemoApp) HandleMouse(ev *tcell.EventMouse) {}

func (a *DemoApp) HandleInterrupt(ev *tcell.EventInterrupt) {}

func (a *DemoApp) HandleResize() {
	c := a.UpdateCanvas()
	w := a.Widgets()
	if len(w) < 3 {
		return
	}
	w[0].SetRect(c.ChildRect(0, 0, c.W(), c.H()))
	w[1].SetRect(c.ChildRect(0, c.H()-2, c.W(), 1))
	w[2].SetRect(c.ChildRect(0, c.H()-1, c.W(), 1))
}

func (a *DemoApp) handleNormalKey(ev *tcell.EventKey) bool {
	if a.tryKeyBindings(a.keyBindings, ev) {
		return true
	}
	if w := a.focusedWidget(); w != nil {
		if h, ok := w.(termui.FocusKeyHandler); ok && h.HandleFocusKey(ev) {
			return true
		}
	}
	return true
}

func (a *DemoApp) handleInsertKey(ev *tcell.EventKey) bool {
	if a.tryKeyBindings(a.insertKeys, ev) {
		return true
	}
	if w := a.focusedWidget(); w != nil {
		w.HandleEvent(ev)
	}
	return true
}

func (a *DemoApp) handleCommandKey(ev *tcell.EventKey) bool {
	if a.cmdWidget == nil {
		return true
	}
	a.cmdWidget.HandleEvent(ev)
	if ev.Key() == tcell.KeyEnter {
		a.cmdWidget.Deativate()
		if a.Mode() == platform.ModeCommand {
			a.SetMode(platform.ModeNormal)
		}
	}
	return true
}

func (a *DemoApp) tryKeyBindings(reg *commands.KeyBindingRegistry, ev *tcell.EventKey) bool {
	if reg == nil {
		return false
	}
	key, ok := platform.KeyFromEvent(ev)
	if !ok {
		reg.ResetPartial()
		return false
	}
	completed, handled := reg.HandleKey(key)
	if !handled {
		return false
	}
	if !completed {
		return reg.InPartial()
	}
	return true
}

func (a *DemoApp) focusedWidget() termui.Widget {
	if a.tab == nil {
		return nil
	}
	return a.tab.FocusedWidget()
}
