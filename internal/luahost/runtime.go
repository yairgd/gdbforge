// Package luahost embeds gopher-lua as the gdbforge script engine.
package luahost

import (
	"fmt"
	"strings"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// Pane is the surface a Lua script draws/prints into (implemented by LuaWidget).
type Pane interface {
	AppendPrint(s string)
	ClearAll()
	ClearCells()
	SetCell(x, y int, ch rune, color string)
	Size() (w, h int)
}

// OnRegister is called when Lua runs gdbforge.register(name, fn).
type OnRegister func(name string, rt *Runtime)

// PrintSinkFunc receives gdbforge.print lines (e.g. OutputWidget). Prefer over pane.
type PrintSinkFunc func(line string)

// Runtime wraps one Lua VM bound to a Pane and optional command registry hook.
type Runtime struct {
	mu              sync.Mutex
	L               *lua.LState
	pane            Pane
	printSink       PrintSinkFunc
	registered      map[string]*lua.LFunction
	onRegister      OnRegister
	openBuffer      OpenBufferFunc
	run             RunFunc
	spawn           SpawnFunc
	openExternalTTY OpenExternalTTYFunc
	spawnTerminal   SpawnTerminalFunc
	scriptDir       string // directory of the loaded user script (lua_dir())
	scriptPath      string // full path of the loaded user script (empty for embedded)
	lastErr         string
}

// New creates a Runtime and installs the gdbforge / pane APIs.
func New(pane Pane, onRegister OnRegister) *Runtime {
	rt := &Runtime{
		L:          lua.NewState(lua.Options{SkipOpenLibs: false}),
		pane:       pane,
		registered: make(map[string]*lua.LFunction),
		onRegister: onRegister,
	}
	rt.installAPI()
	return rt
}

// Close closes the Lua state.
func (rt *Runtime) Close() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.L != nil {
		rt.L.Close()
		rt.L = nil
	}
}

// SetPane updates the print/draw target (usually the owning widget).
// Must not take rt.mu: open_buffer → adopt runs under CallNamed which holds the lock.
func (rt *Runtime) SetPane(pane Pane) {
	rt.pane = pane
}

// SetPrintSink installs a host print destination (e.g. :b io).
// When set, gdbforge.print prefers the sink over pane.AppendPrint.
// Must not take rt.mu: cleared under CallNamed during AdoptLuaWidget.
func (rt *Runtime) SetPrintSink(fn PrintSinkFunc) {
	rt.printSink = fn
}

// SetScriptDir sets the directory returned by gdbforge.lua_dir() (sidecar files).
func (rt *Runtime) SetScriptDir(dir string) {
	rt.mu.Lock()
	rt.scriptDir = dir
	rt.mu.Unlock()
}

// SetScriptPath records the loaded script file path (for pane clone / lazy :b).
func (rt *Runtime) SetScriptPath(path string) {
	rt.mu.Lock()
	rt.scriptPath = path
	rt.mu.Unlock()
}

// ScriptPath returns the loaded script file path, or empty for embedded chunks.
// Must not take rt.mu: may run under CallNamed (open_buffer create-or-focus).
func (rt *Runtime) ScriptPath() string {
	return rt.scriptPath
}

// HasPaneHooks reports whether this script defines on_key or on_tick
// (interactive ModeLua pane, not automation-only).
// Must not take rt.mu: may run under CallNamed (open_buffer create-or-focus).
func (rt *Runtime) HasPaneHooks() bool {
	if rt.L == nil {
		return false
	}
	if v := rt.L.GetGlobal("on_key"); v.Type() == lua.LTFunction {
		return true
	}
	if v := rt.L.GetGlobal("on_tick"); v.Type() == lua.LTFunction {
		return true
	}
	return false
}

// Pane returns the bound draw/print surface (may be nil).
// Must not take rt.mu: may run under CallNamed.
func (rt *Runtime) Pane() Pane {
	return rt.pane
}

// AppendPrint writes a line via print sink or bound pane.
// Must not take rt.mu: host Lua funcs run under CallNamed/LoadString which
// already hold the lock — re-locking deadlocks the UI (e.g. :lua dlv_ext_port).
func (rt *Runtime) AppendPrint(line string) {
	rt.emitPrint(line)
}

func (rt *Runtime) emitPrint(line string) {
	if rt.printSink != nil {
		rt.printSink(line)
		return
	}
	if rt.pane != nil {
		rt.pane.AppendPrint(line)
	}
}

// SetGdbforgeFunc installs or replaces gdbforge.<name> with fn.
// Used by app packages (e.g. luadebug) to add domain APIs without baking them into the host.
func (rt *Runtime) SetGdbforgeFunc(name string, fn lua.LGFunction) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.L == nil || name == "" || fn == nil {
		return
	}
	gf := rt.L.GetGlobal("gdbforge")
	if tbl, ok := gf.(*lua.LTable); ok {
		rt.L.SetField(tbl, name, rt.L.NewFunction(fn))
	}
}

// LastError returns the last Lua error string (empty if none).
func (rt *Runtime) LastError() string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.lastErr
}

func (rt *Runtime) setErr(err error) {
	if err == nil {
		rt.lastErr = ""
		return
	}
	rt.lastErr = err.Error()
}

// LoadString runs a Lua chunk (e.g. embedded game script).
func (rt *Runtime) LoadString(src, name string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.L == nil {
		return fmt.Errorf("lua runtime closed")
	}
	fn, err := rt.L.LoadString(src)
	if err != nil {
		rt.setErr(err)
		return err
	}
	rt.L.Push(fn)
	err = rt.L.PCall(0, lua.MultRet, nil)
	rt.setErr(err)
	return err
}

// RegisteredNames returns sorted-ish names for :lua Tab completion.
func (rt *Runtime) RegisteredNames() []string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	out := make([]string, 0, len(rt.registered))
	for name := range rt.registered {
		out = append(out, name)
	}
	return out
}

// HasNamed reports whether name was gdbforge.register'd.
func (rt *Runtime) HasNamed(name string) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	_, ok := rt.registered[name]
	return ok
}

// CallNamed invokes a registered Lua function with string args.
func (rt *Runtime) CallNamed(name string, args ...string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.L == nil {
		return fmt.Errorf("lua runtime closed")
	}
	fn, ok := rt.registered[name]
	if !ok || fn == nil {
		err := fmt.Errorf("unknown lua command: %s", name)
		rt.setErr(err)
		return err
	}
	rt.L.Push(fn)
	for _, a := range args {
		rt.L.Push(lua.LString(a))
	}
	err := rt.L.PCall(len(args), lua.MultRet, nil)
	rt.setErr(err)
	return err
}

// DispatchKey calls global on_key(key) if present.
func (rt *Runtime) DispatchKey(key string) {
	rt.callGlobal("on_key", lua.LString(key))
}

// DispatchTick calls global on_tick(dt) if present.
func (rt *Runtime) DispatchTick(dt float64) {
	rt.callGlobal("on_tick", lua.LNumber(dt))
}

// DispatchDraw calls global on_draw() if present.
func (rt *Runtime) DispatchDraw() {
	rt.callGlobal("on_draw")
}

func (rt *Runtime) callGlobal(name string, args ...lua.LValue) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.L == nil {
		return
	}
	v := rt.L.GetGlobal(name)
	if v.Type() != lua.LTFunction {
		return
	}
	rt.L.Push(v)
	for _, a := range args {
		rt.L.Push(a)
	}
	err := rt.L.PCall(len(args), 0, nil)
	rt.setErr(err)
}

func (rt *Runtime) installAPI() {
	L := rt.L
	gf := L.NewTable()
	L.SetGlobal("gdbforge", gf)

	// Host API only. Debugger bindings (gdb, dlv_*, set_inferior_tty, program)
	// are installed by the app via gdbforge/luadebug.Install after New.
	L.SetField(gf, "print", L.NewFunction(rt.luaPrint))
	L.SetField(gf, "clear", L.NewFunction(rt.luaClear))
	L.SetField(gf, "register", L.NewFunction(rt.luaRegister))
	L.SetField(gf, "open_buffer", L.NewFunction(rt.luaOpenBuffer))
	L.SetField(gf, "run", L.NewFunction(rt.luaRun))
	L.SetField(gf, "spawn", L.NewFunction(rt.luaSpawn))
	L.SetField(gf, "spawn_terminal", L.NewFunction(rt.luaSpawnTerminal))
	L.SetField(gf, "open_external_tty", L.NewFunction(rt.luaOpenExternalTTY))
	L.SetField(gf, "wait_port", L.NewFunction(rt.luaWaitPort))
	L.SetField(gf, "lua_dir", L.NewFunction(rt.luaLuaDir))
	L.SetField(gf, "sleep", L.NewFunction(rt.luaSleep))

	pane := L.NewTable()
	L.SetGlobal("pane", pane)
	L.SetField(pane, "clear", L.NewFunction(rt.luaPaneClear))
	L.SetField(pane, "set_cell", L.NewFunction(rt.luaSetCell))
	L.SetField(pane, "size", L.NewFunction(rt.luaSize))
}

func (rt *Runtime) luaPrint(L *lua.LState) int {
	n := L.GetTop()
	parts := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		parts = append(parts, L.ToString(i))
	}
	line := strings.Join(parts, "\t")
	rt.emitPrint(line)
	return 0
}

func (rt *Runtime) luaClear(L *lua.LState) int {
	if rt.pane != nil {
		rt.pane.ClearAll()
	}
	return 0
}

func (rt *Runtime) luaRegister(L *lua.LState) int {
	name := L.CheckString(1)
	fn := L.CheckFunction(2)
	rt.registered[name] = fn
	if rt.onRegister != nil {
		rt.onRegister(name, rt)
	}
	return 0
}

func (rt *Runtime) luaPaneClear(L *lua.LState) int {
	if rt.pane != nil {
		rt.pane.ClearCells()
	}
	return 0
}

func (rt *Runtime) luaSetCell(L *lua.LState) int {
	x := L.CheckInt(1)
	y := L.CheckInt(2)
	chStr := L.CheckString(3)
	color := ""
	if L.GetTop() >= 4 {
		color = L.OptString(4, "")
	}
	var ch rune = ' '
	if chStr != "" {
		for _, r := range chStr {
			ch = r
			break
		}
	}
	if rt.pane != nil {
		rt.pane.SetCell(x, y, ch, color)
	}
	return 0
}

func (rt *Runtime) luaSize(L *lua.LState) int {
	w, h := 0, 0
	if rt.pane != nil {
		w, h = rt.pane.Size()
	}
	L.Push(lua.LNumber(w))
	L.Push(lua.LNumber(h))
	return 2
}
