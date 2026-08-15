// Package luahost embeds gopher-lua as the gdbforge script engine.
package luahost

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	lua "github.com/yuin/gopher-lua"
)

// ErrJobCancelled is returned / raised when a Lua job is stopped (Ctrl-C).
var ErrJobCancelled = errors.New("lua job cancelled")

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
	foreground      ForegroundFunc
	openExternalTTY OpenExternalTTYFunc
	spawnTerminal   SpawnTerminalFunc
	scriptDir       string // directory of the loaded user script (lua_dir())
	scriptPath      string // full path of the loaded user script (empty for embedded)
	lastErr         string
	// jobCtx is set for the duration of an async CallNamed (sleep/wait_port/system + L.SetContext).
	jobCtx context.Context
	sysMu  sync.Mutex
	sysCmd *exec.Cmd // active gdbforge.system process (process group); killed on cancel
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

// SetJobContext installs the cancel context for cooperative host APIs during CallNamed.
// Must not take rt.mu: set from the worker around CallNamed which already locks.
func (rt *Runtime) SetJobContext(ctx context.Context) {
	if rt == nil {
		return
	}
	rt.jobCtx = ctx
}

// JobContext returns the active job context, or context.Background().
func (rt *Runtime) JobContext() context.Context {
	if rt == nil || rt.jobCtx == nil {
		return context.Background()
	}
	return rt.jobCtx
}

// KillSystem SIGKILLs the process group of an in-flight gdbforge.system (if any).
// Safe to call from the UI thread while CallNamed holds rt.mu on the worker.
func (rt *Runtime) KillSystem() {
	if rt == nil {
		return
	}
	rt.sysMu.Lock()
	cmd := rt.sysCmd
	rt.sysMu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
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
	return rt.loadStringLocked(src, name, false)
}

// EvalLine runs one REPL line (expression or statement). gdbforge.print uses the
// print sink; non-nil return values are emitted like the Lua interactive prompt.
func (rt *Runtime) EvalLine(src string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	src = normalizeReplLine(src)
	return rt.loadStringLocked(src, "=<lua>", true)
}

// normalizeReplLine applies REPL conveniences gopher-lua rejects as bare chunks
// (e.g. "help" must be "help()" — a lone identifier is a parse error).
func normalizeReplLine(src string) string {
	trimmed := strings.TrimSpace(src)
	if trimmed == "" {
		return src
	}
	if strings.EqualFold(trimmed, "help") {
		return "help()"
	}
	if len(trimmed) > 5 && strings.EqualFold(trimmed[:4], "help") && (trimmed[4] == ' ' || trimmed[4] == '\t') {
		topic := strings.TrimSpace(trimmed[4:])
		if topic != "" {
			return fmt.Sprintf("help(%q)", topic)
		}
		return "help()"
	}
	if isBareIdent(trimmed) {
		return trimmed + "()"
	}
	return src
}

func isBareIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_':
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (rt *Runtime) loadStringLocked(src, name string, emitReturns bool) error {
	if rt.L == nil {
		return fmt.Errorf("lua runtime closed")
	}
	fn, err := rt.L.LoadString(src)
	if err != nil {
		rt.setErr(err)
		return err
	}
	if ctx := rt.jobCtx; ctx != nil {
		rt.L.SetContext(ctx)
		defer rt.L.RemoveContext()
	}
	rt.L.Push(fn)
	err = rt.L.PCall(0, lua.MultRet, nil)
	if err != nil && rt.jobCtx != nil && rt.jobCtx.Err() != nil {
		err = ErrJobCancelled
	}
	if err != nil {
		rt.setErr(err)
		return err
	}
	if emitReturns {
		n := rt.L.GetTop()
		if n > 0 {
			parts := make([]string, 0, n)
			for i := 1; i <= n; i++ {
				if rt.L.Get(i) != lua.LNil {
					parts = append(parts, rt.L.ToString(i))
				}
			}
			rt.L.Pop(n)
			if len(parts) > 0 {
				rt.emitPrint(strings.Join(parts, "\t"))
			}
		}
	}
	rt.setErr(nil)
	return nil
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
	// Abort Lua after cancel even when not blocked in sleep/wait_port/system
	// (e.g. next instruction after a stuck io.popen returns).
	if ctx := rt.jobCtx; ctx != nil {
		rt.L.SetContext(ctx)
		defer rt.L.RemoveContext()
	}
	rt.L.Push(fn)
	for _, a := range args {
		rt.L.Push(lua.LString(a))
	}
	err := rt.L.PCall(len(args), lua.MultRet, nil)
	if err != nil && rt.jobCtx != nil && rt.jobCtx.Err() != nil {
		err = ErrJobCancelled
	}
	rt.setErr(err)
	return err
}

// CallHelp invokes global help() when present (for :lua <name> help).
func (rt *Runtime) CallHelp() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.L == nil {
		return fmt.Errorf("lua runtime closed")
	}
	v := rt.L.GetGlobal("help")
	if v.Type() != lua.LTFunction {
		err := fmt.Errorf("no help() in script")
		rt.setErr(err)
		return err
	}
	rt.L.Push(v)
	err := rt.L.PCall(0, 0, nil)
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
	// TryLock: never block the UI paint/key path on CallNamed (worker job).
	// Skipping a tick/draw/key frame is preferable to freezing the whole TUI.
	if !rt.mu.TryLock() {
		return
	}
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
	L.SetField(gf, "foreground", L.NewFunction(rt.luaForeground))
	L.SetField(gf, "spawn_terminal", L.NewFunction(rt.luaSpawnTerminal))
	L.SetField(gf, "open_external_tty", L.NewFunction(rt.luaOpenExternalTTY))
	L.SetField(gf, "wait_port", L.NewFunction(rt.luaWaitPort))
	L.SetField(gf, "lua_dir", L.NewFunction(rt.luaLuaDir))
	L.SetField(gf, "sleep", L.NewFunction(rt.luaSleep))
	L.SetField(gf, "system", L.NewFunction(rt.luaSystem))
	L.SetField(gf, "help", L.NewFunction(rt.luaHelp))

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
