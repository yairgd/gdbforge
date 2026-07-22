package luahost

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

const (
	// UserLuaDir is the project-local Lua extension directory under .gdbforge.
	UserLuaDir = "lua"
)

// OpenBufferFunc opens a builtin/buffer by name (e.g. "lua", "snake").
type OpenBufferFunc func(name string)

// RunFunc starts a shell/exec session with argv (same as :!).
type RunFunc func(argv []string)

// SpawnFunc starts a background PTY process; return error if start fails.
type SpawnFunc func(argv []string) error

// GDBFunc sends one GDB CLI command (same path as the GDB console).
type GDBFunc func(cmd string)

// SetOpenBuffer installs gdbforge.open_buffer(name) for user scripts.
func (rt *Runtime) SetOpenBuffer(fn OpenBufferFunc) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.openBuffer = fn
	if rt.L == nil {
		return
	}
	gf := rt.L.GetGlobal("gdbforge")
	if tbl, ok := gf.(*lua.LTable); ok {
		rt.L.SetField(tbl, "open_buffer", rt.L.NewFunction(rt.luaOpenBuffer))
	}
}

// SetRun installs gdbforge.run(...) → same path as :! argv.
func (rt *Runtime) SetRun(fn RunFunc) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.run = fn
	if rt.L == nil {
		return
	}
	gf := rt.L.GetGlobal("gdbforge")
	if tbl, ok := gf.(*lua.LTable); ok {
		rt.L.SetField(tbl, "run", rt.L.NewFunction(rt.luaRun))
	}
}

// SetSpawn installs gdbforge.spawn(...) — background exec, no focus steal.
func (rt *Runtime) SetSpawn(fn SpawnFunc) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.spawn = fn
	if rt.L == nil {
		return
	}
	gf := rt.L.GetGlobal("gdbforge")
	if tbl, ok := gf.(*lua.LTable); ok {
		rt.L.SetField(tbl, "spawn", rt.L.NewFunction(rt.luaSpawn))
	}
}

// SetGDB installs gdbforge.gdb(cmd) for console-style GDB sends.
func (rt *Runtime) SetGDB(fn GDBFunc) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.gdb = fn
	if rt.L == nil {
		return
	}
	gf := rt.L.GetGlobal("gdbforge")
	if tbl, ok := gf.(*lua.LTable); ok {
		rt.L.SetField(tbl, "gdb", rt.L.NewFunction(rt.luaGDB))
		rt.L.SetField(tbl, "sleep", rt.L.NewFunction(rt.luaSleep))
		rt.L.SetField(tbl, "lua_dir", rt.L.NewFunction(rt.luaLuaDir))
	}
}

// LoadDir loads every *.lua file under dir. Each file basename (without .lua)
// becomes a :lua command: either via gdbforge.register in the file, or by
// auto-binding global main() / <basename>() to that name.
// Missing dir is a no-op. Returns how many files were loaded.
func (rt *Runtime) LoadDir(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	var firstErr error
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".lua") {
			continue
		}
		cmd := strings.TrimSuffix(name, filepath.Ext(name))
		if cmd == "" {
			continue
		}
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := rt.LoadString(string(src), path); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", path, err)
			}
			continue
		}
		if err := rt.EnsureCommand(cmd); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", path, err)
			}
			continue
		}
		n++
	}
	return n, firstErr
}

// EnsureCommand registers name for :lua if not already registered.
// Prefers an existing gdbforge.register, else global main, else global <name>.
func (rt *Runtime) EnsureCommand(name string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.L == nil {
		return fmt.Errorf("lua runtime closed")
	}
	if _, ok := rt.registered[name]; ok {
		return nil
	}
	var fn *lua.LFunction
	if v := rt.L.GetGlobal("main"); v.Type() == lua.LTFunction {
		fn = v.(*lua.LFunction)
	} else if v := rt.L.GetGlobal(name); v.Type() == lua.LTFunction {
		fn = v.(*lua.LFunction)
	}
	if fn == nil {
		return fmt.Errorf("no main() or %s() / gdbforge.register(%q, …) in script", name, name)
	}
	rt.registered[name] = fn
	if rt.onRegister != nil {
		rt.onRegister(name, rt)
	}
	return nil
}

func (rt *Runtime) luaOpenBuffer(L *lua.LState) int {
	name := L.CheckString(1)
	if rt.openBuffer != nil {
		rt.openBuffer(name)
	}
	return 0
}

func (rt *Runtime) luaRun(L *lua.LState) int {
	n := L.GetTop()
	argv := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		s := strings.TrimSpace(L.ToString(i))
		if s != "" {
			argv = append(argv, s)
		}
	}
	if len(argv) == 0 {
		L.RaiseError("gdbforge.run: need at least one argument")
		return 0
	}
	if rt.run != nil {
		rt.run(argv)
	}
	return 0
}

func (rt *Runtime) luaSpawn(L *lua.LState) int {
	n := L.GetTop()
	argv := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		s := strings.TrimSpace(L.ToString(i))
		if s != "" {
			argv = append(argv, s)
		}
	}
	if len(argv) == 0 {
		L.RaiseError("gdbforge.spawn: need at least one argument")
		return 0
	}
	if rt.spawn == nil {
		L.RaiseError("gdbforge.spawn: not available")
		return 0
	}
	if err := rt.spawn(argv); err != nil {
		L.RaiseError("%s", err.Error())
		return 0
	}
	if rt.pane != nil {
		rt.pane.AppendPrint("spawned: " + strings.Join(argv, " "))
	}
	return 0
}

// wait_port(port [, timeout_sec]) → true if TCP 127.0.0.1:port accepts.
func (rt *Runtime) luaWaitPort(L *lua.LState) int {
	port := L.CheckString(1)
	timeout := 10.0
	if L.GetTop() >= 2 {
		timeout = float64(L.CheckNumber(2))
	}
	if timeout <= 0 {
		timeout = 10
	}
	if timeout > 120 {
		timeout = 120
	}
	addr := net.JoinHostPort("127.0.0.1", port)
	deadline := time.Now().Add(time.Duration(timeout * float64(time.Second)))
	for {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			L.Push(lua.LTrue)
			return 1
		}
		if time.Now().After(deadline) {
			L.Push(lua.LFalse)
			return 1
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (rt *Runtime) luaGDB(L *lua.LState) int {
	cmd := strings.TrimSpace(L.CheckString(1))
	if cmd == "" {
		return 0
	}
	if rt.gdb != nil {
		rt.gdb(cmd)
	}
	return 0
}

func (rt *Runtime) luaSleep(L *lua.LState) int {
	sec := float64(L.CheckNumber(1))
	if sec < 0 {
		sec = 0
	}
	if sec > 60 {
		sec = 60
	}
	time.Sleep(time.Duration(sec * float64(time.Second)))
	return 0
}

func (rt *Runtime) luaLuaDir(L *lua.LState) int {
	L.Push(lua.LString(filepath.Join(".", ".gdbforge", UserLuaDir)))
	return 1
}
