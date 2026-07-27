package main

import (
	"sort"
	"strconv"
	"strings"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/luadebug"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
	luacatalog "github.com/yairgd/gdbforge/lua"
)

// enterLuaMode focuses a Lua pane and routes all keys to it (ModeLua).
func (a *DebuggerApp) enterLuaMode(w *widgets.LuaWidget) {
	if w == nil {
		return
	}
	if a.activeLua != nil && a.activeLua != w {
		a.activeLua.StopTicks()
	}
	a.activeLua = w
	if a.tab != nil {
		a.tab.SetInsertActive(false)
	}
	a.SetMode(platform.ModeLua)
	w.SetFrameRequester(a.RequestFrame)
	w.StartTicks()
	a.RequestFrame()
}

// leaveLuaMode returns to normal mode and stops the active Lua tick loop.
func (a *DebuggerApp) leaveLuaMode() {
	if a.activeLua != nil {
		a.activeLua.StopTicks()
		a.activeLua = nil
	}
	if a.Mode() == platform.ModeLua {
		a.SetMode(platform.ModeNormal)
	}
	a.RequestFrame()
}

func (a *DebuggerApp) handleLuaKey(ev *tcell.EventKey) bool {
	if key, ok := platform.KeyFromEvent(ev); ok && key.Key == tcell.KeyEscape {
		a.leaveLuaMode()
		return true
	}
	w := a.activeLua
	if w == nil {
		if lw, ok := a.focusedWidget().(*widgets.LuaWidget); ok {
			w = lw
			a.activeLua = w
		}
	}
	if w == nil {
		a.leaveLuaMode()
		return true
	}
	w.HandleLuaKey(ev)
	a.RequestFrame()
	return true
}

func (a *DebuggerApp) registerLuaCmd(name string, rt *luahost.Runtime) {
	if name == "" || rt == nil {
		return
	}
	if a.luaCmds == nil {
		a.luaCmds = make(map[string]*luahost.Runtime)
	}
	a.luaCmds[name] = rt
}

// OnLua runs a gdbforge.register'd function: :lua name [args...]
// Pane scripts (on_key/on_tick): :lua snake [bufname] create-or-focuses that
// buffer (default via main() → open_buffer("snake"); :lua snake snake1 → new VM).
func (a *DebuggerApp) OnLua(args ...any) {
	if len(args) == 0 {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("lua").Error("usage: :lua <funcname> [args...]")
		}
		return
	}
	name, _ := args[0].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	rt, err := a.ensureLuaRuntime(name)
	if err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("lua").Error(err.Error())
		}
		return
	}
	if rt == nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("lua").Error("unknown lua command: " + name)
		}
		return
	}
	strArgs := make([]string, 0, len(args)-1)
	for _, arg := range args[1:] {
		if s, ok := arg.(string); ok {
			strArgs = append(strArgs, s)
		}
	}
	if buf := luaPaneInstanceName(rt, strArgs); buf != "" {
		if !a.ensureLuaBuffer(buf, rt) && a.ctx.Log != nil {
			a.ctx.Log.Named("lua").Error("cannot open lua pane: " + buf)
		}
		a.RequestFrame()
		return
	}
	if err := rt.CallNamed(name, strArgs...); err != nil && a.ctx.Log != nil {
		a.ctx.Log.Named("lua").Error(err.Error())
	}
	a.RequestFrame()
}

// luaPaneInstanceName returns the buffer name for :lua <pane> <bufname>, or "".
func luaPaneInstanceName(rt *luahost.Runtime, strArgs []string) string {
	if rt == nil || !rt.HasPaneHooks() || len(strArgs) == 0 {
		return ""
	}
	return strings.TrimSpace(strArgs[0])
}

func (a *DebuggerApp) luaCompletions(prefix string) []string {
	var names []string
	seen := map[string]struct{}{}
	add := func(name string) {
		if name == "" {
			return
		}
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	for name := range a.luaCmds {
		add(name)
	}
	for name := range a.luaPending {
		add(name)
	}
	sort.Strings(names)
	return names
}

func (a *DebuggerApp) maybeEnterLuaBuffer(w interface{}) {
	lw, ok := w.(*widgets.LuaWidget)
	if !ok || lw == nil {
		return
	}
	a.enterLuaMode(lw)
}

func (a *DebuggerApp) focusBufferWidget(w termui.Widget) {
	if w == nil || a.tab == nil {
		return
	}
	if a.swapFocusedWidget(w) {
		a.maybeEnterLuaBuffer(w)
		a.RequestFrame()
		return
	}
	if _, ok := w.(*widgets.LuaWidget); ok {
		a.maybeEnterLuaBuffer(w)
		a.RequestFrame()
	}
}

// loadUserLuaScripts indexes :lua <basename> commands from the 3-layer search
// (project → home → embedded). VMs load lazily on first :lua / ensureLuaRuntime.
func (a *DebuggerApp) loadUserLuaScripts() {
	files, err := luahost.ResolveLuaScripts(luacatalog.FS)
	if err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("lua").Error("resolve lua scripts: " + err.Error())
		}
		return
	}
	if a.luaPending == nil {
		a.luaPending = make(map[string]luahost.ResolvedScript)
	}
	byOrigin := map[string]int{}
	for _, f := range files {
		a.luaPending[f.Cmd] = f
		byOrigin[f.Origin]++
	}
	if a.ctx.Log == nil || len(files) == 0 {
		return
	}
	a.ctx.Log.Named("lua").Info("indexed " + strconv.Itoa(len(files)) + " lua scripts (lazy)" +
		" (project=" + strconv.Itoa(byOrigin[luahost.OriginProject]) +
		" home=" + strconv.Itoa(byOrigin[luahost.OriginHome]) +
		" embedded=" + strconv.Itoa(byOrigin[luahost.OriginEmbedded]) + ")")
}

// ensureLuaRuntime returns a loaded Runtime for cmd, loading from luaPending on first use.
func (a *DebuggerApp) ensureLuaRuntime(cmd string) (*luahost.Runtime, error) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil, nil
	}
	if rt := a.luaCmds[cmd]; rt != nil {
		return rt, nil
	}
	f, ok := a.luaPending[cmd]
	if !ok {
		return nil, nil
	}
	rt := luahost.New(nil, a.registerLuaCmd)
	a.wireUserLuaAPI(rt)
	if err := rt.LoadScriptFile(f.Path, f.Cmd); err != nil {
		rt.Close()
		return nil, err
	}
	a.luaUserRuntimes = append(a.luaUserRuntimes, rt)
	a.luaUser = rt
	delete(a.luaPending, cmd)
	if a.ctx.Log != nil {
		a.ctx.Log.Named("lua").Info(":lua " + f.Cmd + " loaded from " + f.Origin + " (" + f.Path + ")")
	}
	// Load may register extra names (e.g. snake_score); primary cmd is in luaCmds.
	if a.luaCmds[cmd] == nil {
		// EnsureCommand should have registered; recover if script only has main.
		_ = rt.EnsureCommand(cmd)
	}
	return a.luaCmds[cmd], nil
}

// wireUserLuaAPI installs host callbacks shared by every user script Runtime.
func (a *DebuggerApp) wireUserLuaAPI(rt *luahost.Runtime) {
	if rt == nil {
		return
	}
	rt.SetPrintSink(func(line string) {
		if a.outputWidget != nil {
			a.outputWidget.AppendHostLine(line)
			a.RequestFrame()
		}
	})
	rt.SetOpenBuffer(func(name string) {
		a.openBufferForLua(name, rt)
	})
	rt.SetRun(func(argv []string) {
		anyArgs := make([]any, len(argv))
		for i, s := range argv {
			anyArgs[i] = s
		}
		a.OnRun(anyArgs...)
	})
	rt.SetSpawn(func(argv []string) error {
		return a.SpawnExec(argv)
	})
	rt.SetOpenExternalTTY(a.OpenExternalTTY)
	rt.SetSpawnTerminal(a.SpawnTerminal)
	luadebug.Install(rt, luadebug.Hooks{
		SetInferiorTTY:   a.SetInferiorTTY,
		DlvConnect:       a.ConnectDlv,
		SpawnDlvHeadless: a.SpawnDlvHeadless,
		Program:          a.SessionProgram,
		GDB: func(cmd string) {
			cmd = strings.TrimSpace(cmd)
			if cmd == "" {
				return
			}
			if a.gdbWidget != nil {
				a.gdbWidget.EchoSubmit(cmd)
				a.gdbWidget.FollowTailAndScroll()
			}
			if a.isDLV() {
				if a.dlvClient == nil {
					return
				}
				a.withGdbUIOwner(func() { _ = a.dlvClient.Send(cmd) })
			} else {
				if a.gdbClient == nil {
					return
				}
				sendCmd := gdb.CLIExecToMI(cmd)
				a.withGdbUIOwner(func() { _ = a.gdbClient.Send(sendCmd) })
			}
			a.RequestFrame()
		},
	})
}

// openBufferForLua focuses named panes without stealing the Code leaf via swap.
// "code" / "gdb" use leaf marks; other names use create-or-focus for pane scripts.
func (a *DebuggerApp) openBufferForLua(name string, from *luahost.Runtime) {
	name = strings.TrimSpace(name)
	switch name {
	case "code":
		a.leaveLuaMode()
		if cw := a.codeBufferForB(); cw != nil {
			a.placeCodeInSlot(cw)
		}
		a.activateCodePane()
		a.RequestFrame()
	case "gdb":
		// Focus existing GDB leaf only — never relocate GDB onto the Code leaf.
		a.leaveLuaMode()
		if a.tab != nil && a.gdbWidget != nil {
			if leaf := a.findGdbLeaf(); leaf != nil {
				_ = a.tab.FocusLeaf(leaf)
			} else {
				a.tab.FocusWidget(a.gdbWidget)
			}
			a.tab.SetInsertActive(true)
		}
		a.SetMode(platform.ModeInsert)
		a.RequestFrame()
	default:
		a.openOrCreateBuffer(name, from)
	}
}

// openOrCreateBuffer focuses an existing buffer, or creates a Lua pane when
// allowed: from a pane script's open_buffer, or lazy :b for known pane scripts.
func (a *DebuggerApp) openOrCreateBuffer(name string, from *luahost.Runtime) {
	if name == "" || a.tab == nil {
		return
	}
	if w := a.builtins[name]; w != nil {
		a.focusBufferWidget(w)
		return
	}
	if w := a.findFileBuffer(name); w != nil {
		if a.swapFocusedWidget(w) {
			a.RequestFrame()
		}
		return
	}
	if from != nil {
		// Lua open_buffer: create only when the caller is a pane script.
		if from.HasPaneHooks() && a.ensureLuaBuffer(name, from) {
			return
		}
		if a.ctx.Log != nil {
			a.ctx.Log.Named("buffer").Error("no matching buffer: " + name)
		}
		return
	}
	// :b name — only existing builtins/files (pane scripts appear after :lua creates them).
	if a.ctx.Log != nil {
		a.ctx.Log.Named("buffer").Error("no matching buffer: " + name)
	}
}

// ensureLuaBuffer create-or-focuses a LuaWidget for a pane script Runtime.
// First call adopts rt; further names clone from ScriptPath() into a new VM.
func (a *DebuggerApp) ensureLuaBuffer(name string, rt *luahost.Runtime) bool {
	if name == "" || rt == nil || !rt.HasPaneHooks() {
		return false
	}
	if w := a.builtins[name]; w != nil {
		a.focusBufferWidget(w)
		return true
	}

	var w *widgets.LuaWidget
	if owner := a.luaWidgetOwning(rt); owner == nil {
		w = widgets.AdoptLuaWidget(name, rt)
		a.detachUserRuntime(rt)
	} else {
		path := rt.ScriptPath()
		if path == "" {
			if a.ctx.Log != nil {
				a.ctx.Log.Named("lua").Error("cannot clone lua pane " + name + ": no script path")
			}
			return false
		}
		clone := luahost.New(nil, nil)
		a.wireUserLuaAPI(clone)
		if err := clone.LoadScriptFileOnly(path); err != nil {
			clone.Close()
			if a.ctx.Log != nil {
				a.ctx.Log.Named("lua").Error("clone lua pane " + name + ": " + err.Error())
			}
			return false
		}
		w = widgets.AdoptLuaWidget(name, clone)
	}
	w.SetFrameRequester(a.RequestFrame)
	a.registerBuiltin(name, w)
	a.luaDynamic = append(a.luaDynamic, w)
	a.focusBufferWidget(w)
	return true
}

func (a *DebuggerApp) luaWidgetOwning(rt *luahost.Runtime) *widgets.LuaWidget {
	if rt == nil {
		return nil
	}
	for _, w := range a.luaDynamic {
		if w != nil && w.Runtime() == rt {
			return w
		}
	}
	return nil
}

// detachUserRuntime removes rt from luaUserRuntimes so Close won't double-free
// after the runtime is adopted by a LuaWidget.
func (a *DebuggerApp) detachUserRuntime(rt *luahost.Runtime) {
	if rt == nil || len(a.luaUserRuntimes) == 0 {
		return
	}
	out := a.luaUserRuntimes[:0]
	for _, r := range a.luaUserRuntimes {
		if r != rt {
			out = append(out, r)
		}
	}
	a.luaUserRuntimes = out
	if a.luaUser == rt {
		a.luaUser = nil
		if n := len(out); n > 0 {
			a.luaUser = out[n-1]
		}
	}
}
