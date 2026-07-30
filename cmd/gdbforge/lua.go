package main

import (
	"context"
	"sort"
	"strconv"
	"strings"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdbforge/luadebug"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
	luacatalog "github.com/yairgd/gdbforge/lua"
)

// luaJobDoneMsg is posted when an async :lua CallNamed finishes.
type luaJobDoneMsg struct {
	name string
	err  error
}

// luaUIMsg runs fn on the UI thread (worker → PostEvent → HandleInterrupt).
type luaUIMsg struct {
	fn   func()
	done chan struct{}
}

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
// :lua name help|-h|--help calls global help() (if any) and skips main().
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
	if isLuaHelpRequest(strArgs) {
		if err := rt.CallHelp(); err != nil {
			rt.AppendPrint("no help() for " + name)
			if a.ctx.Log != nil {
				a.ctx.Log.Named("lua").Error(err.Error())
			}
		}
		a.RequestFrame()
		return
	}
	if buf := luaPaneInstanceName(rt, strArgs); buf != "" {
		if !a.ensureLuaBuffer(buf, rt) && a.ctx.Log != nil {
			a.ctx.Log.Named("lua").Error("cannot open lua pane: " + buf)
		}
		a.RequestFrame()
		return
	}
	a.startLuaJob(rt, name, strArgs)
}

// startLuaJob runs CallNamed on a worker so the UI stays responsive.
// One job at a time; Ctrl-C cancels context + kills gdbforge.system children.
func (a *DebuggerApp) startLuaJob(rt *luahost.Runtime, name string, strArgs []string) {
	if rt == nil || name == "" {
		return
	}
	if a.luaJobBusy.Load() {
		rt.AppendPrint("lua job already running — Ctrl-C to cancel")
		a.RequestFrame()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.luaJobMu.Lock()
	a.luaJobCancel = cancel
	a.luaJobRT = rt
	a.luaJobBusy.Store(true)
	a.luaJobMu.Unlock()

	rt.SetJobContext(ctx)
	go func() {
		a.luaOnWorker.Store(true)
		err := rt.CallNamed(name, strArgs...)
		a.luaOnWorker.Store(false)

		a.luaJobMu.Lock()
		a.luaJobCancel = nil
		a.luaJobRT = nil
		a.luaJobBusy.Store(false)
		a.luaJobMu.Unlock()
		rt.SetJobContext(nil)

		if scr := a.Screen(); scr != nil {
			_ = scr.PostEvent(tcell.NewEventInterrupt(luaJobDoneMsg{name: name, err: err}))
		}
	}()
	a.RequestFrame()
}

// cancelLuaJob cancels the in-flight :lua worker job. Returns true if one was active.
func (a *DebuggerApp) cancelLuaJob() bool {
	a.luaJobMu.Lock()
	cancel := a.luaJobCancel
	rt := a.luaJobRT
	a.luaJobMu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	if rt != nil {
		rt.KillSystem()
	}
	return true
}

// callOnUI runs fn on the UI thread when invoked from the Lua worker.
// Synchronous host APIs (open_buffer, gdb echo) need this to avoid racing the tree.
func (a *DebuggerApp) callOnUI(fn func()) {
	if fn == nil {
		return
	}
	if !a.luaOnWorker.Load() {
		fn()
		return
	}
	scr := a.Screen()
	if scr == nil {
		fn()
		return
	}
	done := make(chan struct{})
	msg := luaUIMsg{fn: fn, done: done}
	if err := scr.PostEvent(tcell.NewEventInterrupt(msg)); err != nil {
		fn()
		return
	}
	<-done
}

// isLuaHelpRequest is true for a sole rest arg help / -h / --help.
func isLuaHelpRequest(strArgs []string) bool {
	if len(strArgs) != 1 {
		return false
	}
	switch strings.TrimSpace(strArgs[0]) {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

// luaPaneInstanceName returns the buffer name for :lua <pane> <bufname>, or "".
func luaPaneInstanceName(rt *luahost.Runtime, strArgs []string) string {
	if rt == nil || !rt.HasPaneHooks() || len(strArgs) == 0 {
		return ""
	}
	return strings.TrimSpace(strArgs[0])
}

func (a *DebuggerApp) luaCompletions(prefix string) []string {
	fields := strings.Fields(prefix)
	trailingSpace := len(prefix) > 0 && (prefix[len(prefix)-1] == ' ' || prefix[len(prefix)-1] == '\t')

	// After a known script name, complete the next arg with help / -h / --help.
	// Sync() passes the whole rest string as prefix (e.g. "remotegdb he");
	// CmdWidget.replaceToken only replaces the last whitespace-separated token.
	if len(fields) >= 1 && a.luaScriptKnown(fields[0]) {
		switch {
		case len(fields) == 1 && trailingSpace:
			return []string{"help"}
		case len(fields) >= 2 && !trailingSpace:
			last := fields[len(fields)-1]
			var out []string
			for _, h := range []string{"help", "-h", "--help"} {
				if strings.HasPrefix(h, last) {
					out = append(out, h)
				}
			}
			return out
		case len(fields) >= 2 && trailingSpace:
			return nil
		}
	}

	return a.luaScriptNameCompletions(prefix)
}

func (a *DebuggerApp) luaScriptKnown(name string) bool {
	if name == "" {
		return false
	}
	if _, ok := a.luaCmds[name]; ok {
		return true
	}
	_, ok := a.luaPending[name]
	return ok
}

func (a *DebuggerApp) luaScriptNameCompletions(prefix string) []string {
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
		a.callOnUI(func() { a.openBufferForLua(name, rt) })
	})
	rt.SetRun(func(argv []string) {
		a.callOnUI(func() {
			anyArgs := make([]any, len(argv))
			for i, s := range argv {
				anyArgs[i] = s
			}
			a.OnRun(anyArgs...)
		})
	})
	rt.SetSpawn(func(argv []string) error {
		return a.SpawnExec(argv)
	})
	rt.SetOpenExternalTTY(a.OpenExternalTTY)
	rt.SetSpawnTerminal(a.SpawnTerminal)
	luadebug.Install(rt, luadebug.Hooks{
		SetInferiorTTY: func(path string) error {
			var err error
			a.callOnUI(func() { err = a.SetInferiorTTY(path) })
			return err
		},
		DlvConnect: func(addr string) error {
			var err error
			a.callOnUI(func() { err = a.ConnectDlv(addr) })
			return err
		},
		SpawnDlvHeadless: a.SpawnDlvHeadless,
		Program:          a.SessionProgram,
		GDB: func(cmd string) {
			a.callOnUI(func() {
				cmd = strings.TrimSpace(cmd)
				if cmd == "" {
					return
				}
				if a.gdbWidget != nil {
					a.gdbWidget.EchoSubmit(cmd)
					a.gdbWidget.ForceFollowTailAndScroll()
				}
				if a.backend == nil {
					return
				}
				sendCmd, _ := a.backend.MapExec(cmd)
				a.withGdbUIOwner(func() { _ = a.backend.SendLine(sendCmd) })
				a.RequestFrame()
			})
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
