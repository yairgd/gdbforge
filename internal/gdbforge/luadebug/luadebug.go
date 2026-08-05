// Package luadebug installs debugger-only Lua APIs onto a luahost.Runtime.
// Host package stays free of GDB/Delve/inferior bindings.
package luadebug

import (
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/yairgd/gdbforge/internal/luahost"
)

// Hooks are debugger-app callbacks for Lua.
type Hooks struct {
	SetInferiorTTY   func(path string) error
	DlvConnect       func(addr string) error
	SpawnDlvHeadless func(port string, extraArgs []string) error
	Program          func() string
	CurrentFile      func() string
	CurrentLine      func() int
	StopFile         func() string
	StopLine         func() int
	GDB              func(cmd string)
}

// Install registers gdbforge.set_inferior_tty, dlv_connect, spawn_dlv_headless,
// program, current_file, current_line, stop_file, stop_line, and gdb on rt.
// Safe to call after luahost.New; no-op fields raise "not available" (or return
// "" / 0 for program / current_* / stop_* / no-op for gdb).
func Install(rt *luahost.Runtime, h Hooks) {
	if rt == nil {
		return
	}
	rt.SetGdbforgeFunc("set_inferior_tty", func(L *lua.LState) int {
		path := strings.TrimSpace(L.CheckString(1))
		if h.SetInferiorTTY == nil {
			L.RaiseError("gdbforge.set_inferior_tty: not available")
			return 0
		}
		if err := h.SetInferiorTTY(path); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		rt.AppendPrint("inferior tty: " + path)
		return 0
	})
	rt.SetGdbforgeFunc("dlv_connect", func(L *lua.LState) int {
		addr := strings.TrimSpace(L.CheckString(1))
		if h.DlvConnect == nil {
			L.RaiseError("gdbforge.dlv_connect: not available")
			return 0
		}
		if err := h.DlvConnect(addr); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		rt.AppendPrint("dlv_connect: " + addr)
		return 0
	})
	rt.SetGdbforgeFunc("spawn_dlv_headless", func(L *lua.LState) int {
		port := "2345"
		var extra []string
		n := L.GetTop()
		if n >= 1 {
			port = strings.TrimSpace(L.ToString(1))
		}
		if port == "" {
			port = "2345"
		}
		for i := 2; i <= n; i++ {
			s := strings.TrimSpace(L.ToString(i))
			if s != "" {
				extra = append(extra, s)
			}
		}
		if h.SpawnDlvHeadless == nil {
			L.RaiseError("gdbforge.spawn_dlv_headless: not available")
			return 0
		}
		if err := h.SpawnDlvHeadless(port, extra); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		msg := "spawn_dlv_headless :" + port
		if len(extra) > 0 {
			msg += " -- " + strings.Join(extra, " ")
		}
		rt.AppendPrint(msg)
		return 0
	})
	rt.SetGdbforgeFunc("program", func(L *lua.LState) int {
		if h.Program == nil {
			L.Push(lua.LString(""))
			return 1
		}
		L.Push(lua.LString(h.Program()))
		return 1
	})
	rt.SetGdbforgeFunc("current_file", func(L *lua.LState) int {
		if h.CurrentFile == nil {
			L.Push(lua.LString(""))
			return 1
		}
		L.Push(lua.LString(h.CurrentFile()))
		return 1
	})
	rt.SetGdbforgeFunc("current_line", func(L *lua.LState) int {
		if h.CurrentLine == nil {
			L.Push(lua.LNumber(0))
			return 1
		}
		L.Push(lua.LNumber(h.CurrentLine()))
		return 1
	})
	rt.SetGdbforgeFunc("stop_file", func(L *lua.LState) int {
		if h.StopFile == nil {
			L.Push(lua.LString(""))
			return 1
		}
		L.Push(lua.LString(h.StopFile()))
		return 1
	})
	rt.SetGdbforgeFunc("stop_line", func(L *lua.LState) int {
		if h.StopLine == nil {
			L.Push(lua.LNumber(0))
			return 1
		}
		L.Push(lua.LNumber(h.StopLine()))
		return 1
	})
	rt.SetGdbforgeFunc("gdb", func(L *lua.LState) int {
		cmd := strings.TrimSpace(L.CheckString(1))
		if cmd == "" {
			return 0
		}
		if h.GDB != nil {
			h.GDB(cmd)
		}
		return 0
	})
}
