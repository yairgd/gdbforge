package luahost

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

const (
	// UserLuaDir is the project-local Lua extension directory under .gdbforge.
	UserLuaDir = "lua"
)

// OpenBufferFunc opens a builtin/buffer by name (e.g. "lua", "snake").
type OpenBufferFunc func(name string)

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
