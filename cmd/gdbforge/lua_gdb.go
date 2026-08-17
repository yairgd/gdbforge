package main

import (
	"context"
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	lua "github.com/yuin/gopher-lua"

	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/luahost"
)

// echoGdbWidget shows cmd in :b gdb (async from Lua worker — no deadlock).
func (c *luaCtl) echoGdbWidget(a *DebuggerApp, cmd string) {
	if a == nil || a.gdbWidget == nil {
		return
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	fn := func() {
		a.gdbWidget.EchoSubmit(cmd)
		a.gdbWidget.ForceFollowTailAndScroll()
		a.maybeEnableRemoteMode(cmd)
		a.RequestFrame()
	}
	if !c.onWorker.Load() {
		fn()
		return
	}
	scr := a.Screen()
	if scr == nil {
		fn()
		return
	}
	_ = scr.PostEvent(tcell.NewEventInterrupt(luaUIMsg{fn: fn}))
}

func (c *luaCtl) installGdbAPI(rt *luahost.Runtime) {
	if rt == nil {
		return
	}
	a := c.app
	rt.SetGdbforgeFunc("set_kgdb_mode", func(L *lua.LState) int {
		on := true
		if L.GetTop() >= 1 {
			on = lua.LVAsBool(L.Get(1))
		}
		a.setKgdbMode(on)
		return 0
	})
	rt.SetGdbforgeFunc("gdb_ctrl_c", func(L *lua.LState) int {
		c.echoGdbWidget(a, "^C")
		if a.backend != nil {
			running := a.Debug() != nil && a.Debug().InferiorRunning()
			_ = a.backend.Interrupt(running, false)
		}
		a.RequestFrame()
		return 0
	})
	rt.SetGdbforgeFunc("gdb_query", func(L *lua.LState) int {
		cmd := strings.TrimSpace(L.CheckString(1))
		if cmd == "" {
			L.Push(lua.LString(""))
			L.Push(lua.LString("empty gdb command"))
			return 2
		}
		if a.gdbMcp == nil {
			L.Push(lua.LString(""))
			L.Push(lua.LString("gdb_query: no gdb session"))
			return 2
		}
		c.echoGdbWidget(a, cmd)
		timeout := 120.0
		if L.GetTop() >= 2 {
			timeout = float64(L.CheckNumber(2))
		}
		if timeout <= 0 {
			timeout = 120
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout*float64(time.Second)))
		defer cancel()
		var out string
		var err error
		if gdb.IsTargetRemoteCmd(cmd) {
			out, err = a.gdbMcp.QueryLong(ctx, cmd)
		} else {
			out, err = a.gdbMcp.Query(ctx, cmd)
		}
		if err != nil {
			L.Push(lua.LString(""))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(out))
		L.Push(lua.LString(""))
		return 2
	})
}
