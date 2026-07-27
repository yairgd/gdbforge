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

// OpenExternalTTYFunc opens a real terminal holding a pts and returns its path.
type OpenExternalTTYFunc func() (string, error)

// SpawnTerminalFunc opens a real terminal emulator running argv.
type SpawnTerminalFunc func(argv []string) error

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

// SetOpenExternalTTY installs gdbforge.open_external_tty() → pts path.
func (rt *Runtime) SetOpenExternalTTY(fn OpenExternalTTYFunc) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.openExternalTTY = fn
	if rt.L == nil {
		return
	}
	gf := rt.L.GetGlobal("gdbforge")
	if tbl, ok := gf.(*lua.LTable); ok {
		rt.L.SetField(tbl, "open_external_tty", rt.L.NewFunction(rt.luaOpenExternalTTY))
	}
}

// SetSpawnTerminal installs gdbforge.spawn_terminal(...) — real terminal + argv.
func (rt *Runtime) SetSpawnTerminal(fn SpawnTerminalFunc) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.spawnTerminal = fn
	if rt.L == nil {
		return
	}
	gf := rt.L.GetGlobal("gdbforge")
	if tbl, ok := gf.(*lua.LTable); ok {
		rt.L.SetField(tbl, "spawn_terminal", rt.L.NewFunction(rt.luaSpawnTerminal))
	}
}

// ScriptFile is one discoverable *.lua under a user lua tree.
type ScriptFile struct {
	Path string // absolute or relative path to the .lua file
	Cmd  string // :lua command name (basename without .lua)
}

// WalkLuaScripts recursively finds *.lua under root. Command name is the file
// basename (e.g. games/snake/snake.lua → "snake"). Missing root is empty.
func WalkLuaScripts(root string) ([]ScriptFile, error) {
	var out []ScriptFile
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == root {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".lua") {
			return nil
		}
		cmd := strings.TrimSuffix(name, filepath.Ext(name))
		if cmd == "" {
			return nil
		}
		out = append(out, ScriptFile{Path: path, Cmd: cmd})
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

// LoadDir loads every *.lua file under dir (non-recursive). Prefer WalkLuaScripts
// + per-file Runtime for nested trees (scripts/ copied into .gdbforge/lua/).
// Each file basename (without .lua) becomes a :lua command via EnsureCommand.
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
		if err := rt.LoadScriptFile(path, cmd); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		n++
	}
	return n, firstErr
}

// LoadScriptFile reads path, runs it, and EnsureCommand(cmd). Sets scriptDir
// to the file's directory for gdbforge.lua_dir().
func (rt *Runtime) LoadScriptFile(path, cmd string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rt.SetScriptDir(filepath.Dir(path))
	if err := rt.LoadString(string(src), path); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := rt.EnsureCommand(cmd); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
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

func (rt *Runtime) luaOpenExternalTTY(L *lua.LState) int {
	if rt.openExternalTTY == nil {
		L.RaiseError("gdbforge.open_external_tty: not available")
		return 0
	}
	path, err := rt.openExternalTTY()
	if err != nil {
		L.RaiseError("%s", err.Error())
		return 0
	}
	if rt.pane != nil {
		rt.pane.AppendPrint("external tty: " + path)
	}
	L.Push(lua.LString(path))
	return 1
}

func (rt *Runtime) luaSpawnTerminal(L *lua.LState) int {
	n := L.GetTop()
	argv := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		s := strings.TrimSpace(L.ToString(i))
		if s != "" {
			argv = append(argv, s)
		}
	}
	if len(argv) == 0 {
		L.RaiseError("gdbforge.spawn_terminal: need at least one argument")
		return 0
	}
	if rt.spawnTerminal == nil {
		L.RaiseError("gdbforge.spawn_terminal: not available")
		return 0
	}
	if err := rt.spawnTerminal(argv); err != nil {
		L.RaiseError("%s", err.Error())
		return 0
	}
	if rt.pane != nil {
		rt.pane.AppendPrint("spawn_terminal: " + strings.Join(argv, " "))
	}
	return 0
}

// wait_port(host_port [, timeout_sec]) → true if TCP accepts.
// host_port may be "1234" (→ 127.0.0.1:1234) or "192.168.20.50:1234".
func (rt *Runtime) luaWaitPort(L *lua.LState) int {
	spec := strings.TrimSpace(L.CheckString(1))
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
	host, port, err := net.SplitHostPort(spec)
	if err != nil {
		// Bare port → localhost.
		host = "127.0.0.1"
		port = spec
	}
	if host == "" {
		host = "127.0.0.1"
	}
	addr := net.JoinHostPort(host, port)
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
	// Do not lock: CallNamed holds rt.mu while running Lua.
	dir := rt.scriptDir
	if dir == "" {
		dir = filepath.Join(".", GdbforgeDir, UserLuaDir)
	}
	L.Push(lua.LString(dir))
	return 1
}
