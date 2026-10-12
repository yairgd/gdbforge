package widgets

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/termforge"
)

// handleListMouse is the mouse handling shared by the debug list panes
// (threads, call stack, breakpoints): the wheel moves and activates, a press on
// a row selects it, and the release after that press activates. *down tracks
// the press across events so drag samples do not activate.
func handleListMouse(tw *termforge.TableWidget, down *bool, e *tcell.EventMouse, move func(int), activate func()) {
	if tw.TryDoubleClickWord(e) {
		return
	}
	btns := e.Buttons()
	switch {
	case btns&tcell.WheelUp != 0:
		move(-1)
		activate()
	case btns&tcell.WheelDown != 0:
		move(1)
		activate()
	case btns&tcell.ButtonPrimary != 0:
		if row, ok := tw.HitDataRow(e.Position()); ok {
			tw.SetSelectedRow(row)
			*down = true
		}
	case *down:
		*down = false
		if row, ok := tw.HitDataRow(e.Position()); ok {
			tw.SetSelectedRow(row)
		}
		activate()
	}
}
