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
func (c *luaCtl) echoGdbWidget(h luaHost, cmd string) {
	if h == nil || h.GDBWidget() == nil {
		return
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	fn := func() {
		h.GDBWidget().EchoSubmit(cmd)
		h.GDBWidget().ForceFollowTailAndScroll()
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

func (c *luaCtl) installGdbAPI(rt *luahost.Runtime) {
	if rt == nil {
		return
	}
	h := c.host
	rt.SetGdbforgeFunc("set_kgdb_mode", func(L *lua.LState) int {
		on := true
		if L.GetTop() >= 1 {
			on = lua.LVAsBool(L.Get(1))
		}
		h.SetKgdbMode(on)
		return 0
	})
	rt.SetGdbforgeFunc("gdb_ctrl_c", func(L *lua.LState) int {
		c.echoGdbWidget(h, "^C")
		if b := h.Backend(); b != nil {
			running := h.Debug() != nil && h.Debug().InferiorRunning()
			_ = b.Interrupt(running, false)
		}
		h.RequestFrame()
		return 0
	})
	rt.SetGdbforgeFunc("gdb_query", func(L *lua.LState) int {
		cmd := strings.TrimSpace(L.CheckString(1))
		if cmd == "" {
			L.Push(lua.LString(""))
			L.Push(lua.LString("empty gdb command"))
			return 2
		}
		mcp := h.GdbMcp()
		if mcp == nil {
			L.Push(lua.LString(""))
			L.Push(lua.LString("gdb_query: no gdb session"))
			return 2
		}
		c.echoGdbWidget(h, cmd)
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
			out, err = mcp.QueryLong(ctx, cmd)
		} else {
			out, err = mcp.Query(ctx, cmd)
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
