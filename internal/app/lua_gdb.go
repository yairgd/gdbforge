package app

import (
	"strings"

	tcell "github.com/gdamore/tcell/v2"
)

// echoGdbWidget shows cmd in :b gdb (async from Lua worker — no deadlock).
func (c *luaCtl) echoGdbWidget(h luaHost, cmd string) {
	if h == nil || h.GDBWidget() == nil {
		return
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	fn := func() {
		h.GDBWidget().AppendHostLine(">>> " + cmd)
		h.MaybeEnableRemoteMode(cmd)
		h.RequestFrame()
	}
	if !c.onWorker.Load() {
		fn()
		return
	}
	scr := h.Screen()
	if scr == nil {
		fn()
		return
	}
	_ = scr.PostEvent(tcell.NewEventInterrupt(luaUIMsg{fn: fn}))
}
