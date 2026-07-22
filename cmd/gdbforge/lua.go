package main

import (
	"sort"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/platform"
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
	rt := a.luaCmds[name]
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
	if err := rt.CallNamed(name, strArgs...); err != nil && a.ctx.Log != nil {
		a.ctx.Log.Named("lua").Error(err.Error())
	}
	a.RequestFrame()
}

func (a *DebuggerApp) luaCompletions(prefix string) []string {
	var names []string
	seen := map[string]struct{}{}
	for name := range a.luaCmds {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
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
