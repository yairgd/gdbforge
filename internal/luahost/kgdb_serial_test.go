package luahost

import (
	"os"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestKgdbSerialMainBoardTTY(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "lua", "kgdb_serial", "kgdb_serial.lua")
	src, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}

	rt := New(nil, nil)
	defer rt.Close()
	rt.SetPrintSink(func(string) {})
	rt.SetGdbforgeFunc("print", func(L *lua.LState) int { return 0 })
	rt.SetGdbforgeFunc("open_serial_terminal", func(L *lua.LState) int { return 0 })
	rt.SetGdbforgeFunc("serial_terminal_pty", func(L *lua.LState) int {
		L.Push(lua.LString("/dev/pts/1"))
		return 1
	})
	rt.SetGdbforgeFunc("serial_debugger_pty", func(L *lua.LState) int {
		L.Push(lua.LString("/dev/pts/2"))
		return 1
	})
	rt.SetGdbforgeFunc("spawn_terminal", func(L *lua.LState) int { return 0 })
	rt.SetGdbforgeFunc("spawn_serial_console", func(L *lua.LState) int { return 0 })

	if err := rt.LoadString(string(src), script); err != nil {
		t.Fatal(err)
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	fn := rt.L.GetGlobal("main")
	if fn.Type() != lua.LTFunction {
		t.Fatal("main not a function")
	}
	rt.L.Push(fn)
	if err := rt.L.PCall(0, 0, nil); err != nil {
		t.Fatal(err)
	}
}
