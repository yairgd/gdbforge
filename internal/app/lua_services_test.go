package app

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/yairgd/gdbforge/internal/luahost"
)

func TestServicesRegistry(t *testing.T) {
	a := &DebuggerApp{}
	a.lua.host = a
	reg := a.Services()
	if reg != a.Services() {
		t.Fatal("registry must be built once")
	}
	var local, remote []string
	for _, s := range reg.List() {
		if s.Remote {
			remote = append(remote, s.Name)
		} else {
			local = append(local, s.Name)
		}
	}
	sort.Strings(remote)
	wantRemote := "clear_breakpoint current_file current_line debugger_path evaluate gdb_command list_breakpoints list_frames list_threads program read_memory registers run_control set_breakpoint stop_file stop_line"
	if got := strings.Join(remote, " "); got != wantRemote {
		t.Fatalf("remote services:\n got %s\nwant %s", got, wantRemote)
	}
	for _, n := range []string{"gdb", "gdb_query", "gdb_ctrl_c", "set_kgdb_mode", "uart_send", "dlv_connect", "set_inferior_tty",
		"spawn_dlv_headless", "open_serial_terminal", "serial_send", "serial_owner", "spawn_serial_console"} {
		found := false
		for _, l := range local {
			found = found || l == n
		}
		if !found {
			t.Errorf("missing local service %s", n)
		}
	}
}

// gdbforge.help() is hand-written; every service needs an entry there.
func TestServicesHaveLuaHelp(t *testing.T) {
	a := &DebuggerApp{}
	a.lua.host = a
	help := strings.Join(luahost.APIHelp(""), "\n")
	for _, s := range a.Services().List() {
		if !strings.Contains(help, "gdbforge."+s.Name+"(") {
			t.Errorf("api_help.txt has no entry for gdbforge.%s", s.Name)
		}
	}
}

// The old call shapes scripts use must still work on top of the services.
func TestLuaServiceCallShapes(t *testing.T) {
	a := &DebuggerApp{}
	a.lua.host = a
	rt := luahost.New(&discardPane{}, nil)
	defer rt.Close()
	luahost.InstallServices(rt, a.Services())
	if err := rt.LoadString(luaPrelude, "prelude"); err != nil {
		t.Fatal(err)
	}
	if err := rt.LoadString(`
q_out, q_err = gdbforge.gdb_query("info threads", 5)
ok2, e2 = pcall(gdbforge.uart_send, "", "line")
ok3, e3 = pcall(gdbforge.uart_send, "", 9600, "line")
prog = gdbforge.program()
`, "shapes"); err != nil {
		t.Fatal(err)
	}
	str := func(name string) string { return rt.L.GetGlobal(name).String() }
	if str("q_out") != "" || str("q_err") != "gdb_query: no gdb session" {
		t.Fatalf("gdb_query must return (out, err): %q, %q", str("q_out"), str("q_err"))
	}
	for _, n := range []string{"e2", "e3"} {
		if !strings.Contains(str(n), "empty device") {
			t.Fatalf("uart_send shape %s: %q", n, str(n))
		}
	}
	if str("prog") != "" {
		t.Fatalf("program=%q", str("prog"))
	}
}

func TestOnUIFromLuaRunsInline(t *testing.T) {
	a := &DebuggerApp{}
	a.lua.host = a
	ran := false
	if err := a.lua.onUI(luahost.WithLua(context.Background()), func() { ran = true }); err != nil || !ran {
		t.Fatalf("ran=%v err=%v", ran, err)
	}
}

func TestMIQuoteAndDlvClean(t *testing.T) {
	if got := miQuote(`a "b" \c`); got != `"a \"b\" \\c"` {
		t.Fatalf("%s", got)
	}
	out := "print n\r\n42\r\n(dlv) \r\n"
	if got := cleanDlvOutput(out, "print n"); got != "42" {
		t.Fatalf("%q", got)
	}
}

func TestRunControlNeedsSession(t *testing.T) {
	a := &DebuggerApp{}
	if err := a.RunControl("continue"); err == nil {
		t.Fatal("no session must fail")
	}
}

func TestCleanDlvOutputError(t *testing.T) {
	out := "print zz\r\nCommand failed: could not find symbol value for zz\r\n(dlv) "
	if got := cleanDlvOutput(out, "print zz"); got != "Command failed: could not find symbol value for zz" {
		t.Fatalf("%q", got)
	}
}
