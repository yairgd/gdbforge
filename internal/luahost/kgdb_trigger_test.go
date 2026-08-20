package luahost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestKgdbTriggerSimple(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	trigger := filepath.Join(root, "lua", "kgdb_trigger", "kgdb_trigger.lua")

	var log []string
	rt := New(nil, nil)
	defer rt.Close()
	rt.SetPrintSink(func(line string) { log = append(log, line) })
	rt.SetGdbforgeFunc("print", func(L *lua.LState) int { return 0 })
	rt.SetGdbforgeFunc("serial_debugger_pty", func(L *lua.LState) int {
		L.Push(lua.LString("/dev/pts/99"))
		return 1
	})
	rt.SetGdbforgeFunc("serial_send", func(L *lua.LState) int {
		log = append(log, "sysrq:"+L.CheckString(1))
		return 0
	})
	rt.SetGdbforgeFunc("serial_switch_gdb", func(L *lua.LState) int {
		log = append(log, "switch_gdb")
		return 0
	})
	rt.SetGdbforgeFunc("set_kgdb_mode", func(L *lua.LState) int {
		log = append(log, "kgdb_mode")
		return 0
	})

	if err := rt.LoadString(mustRead(t, trigger), trigger); err != nil {
		t.Fatal(err)
	}
	if err := rt.EnsureCommand("kgdb_trigger"); err != nil {
		t.Fatal(err)
	}
	if err := rt.CallNamed("kgdb_trigger"); err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(log, "|")
	sr := strings.Index(joined, "sysrq:")
	sg := strings.Index(joined, "switch_gdb")
	if sr < 0 || sg < 0 || sg < sr {
		t.Fatalf("want sysrq then switch_gdb, got %s", joined)
	}
	if strings.Contains(joined, "target remote") {
		t.Fatalf("must not target remote: %s", joined)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
