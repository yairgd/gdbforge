package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdbforge/luadebug"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/platform"
	luacatalog "github.com/yairgd/gdbforge/lua"
)

// luaCtl owns Lua scripting state: pane widgets, script registry, and async jobs.
// Mode registration and buffer focus helpers that other subsystems use stay on
// *DebuggerApp and call into luaCtl.
type luaCtl struct {
	app *DebuggerApp

	dynamic      []*widgets.LuaWidget // create-or-focus panes (:lua games, …)
	active       *widgets.LuaWidget
	cmds         map[string]*luahost.Runtime
	pending      map[string]luahost.ResolvedScript // indexed at boot; loaded on first :lua
	user         *luahost.Runtime
	userRuntimes []*luahost.Runtime
	repl         *luahost.Runtime // shared :b lua / :lua console REPL VM
	jobMu        sync.Mutex
	jobCancel    context.CancelFunc
	jobRT        *luahost.Runtime // runtime running the current job (for KillSystem)
	jobBusy      atomic.Bool
	onWorker     atomic.Bool // true while CallNamed runs on the worker goroutine
}

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

// enterMode focuses a Lua pane and routes all keys to it (ModeLua).
func (c *luaCtl) enterMode(w *widgets.LuaWidget) {
	a := c.app
	if w == nil {
		return
	}
	if c.active != nil && c.active != w {
		c.active.StopTicks()
	}
	c.active = w
	if a.Tab() != nil {
		a.Tab().SetInsertActive(false)
	}
	a.SetMode(platform.ModeLua)
	w.SetFrameRequester(a.RequestFrame)
	w.StartTicks()
	a.RequestFrame()
}

// leaveMode returns to normal mode and stops the active Lua tick loop.
func (c *luaCtl) leaveMode() {
	a := c.app
	if c.active != nil {
		c.active.StopTicks()
		c.active = nil
	}
	if a.Mode() == platform.ModeLua {
		a.SetMode(platform.ModeNormal)
	}
	a.RequestFrame()
}

func (c *luaCtl) handleKey(ev *tcell.EventKey) bool {
	a := c.app
	if key, ok := platform.KeyFromEvent(ev); ok && key.Key == tcell.KeyEscape {
		c.leaveMode()
		return true
	}
	w := c.active
	if w == nil {
		if lw, ok := a.focusedWidget().(*widgets.LuaWidget); ok {
			w = lw
			c.active = w
		}
	}
	if w == nil {
		c.leaveMode()
		return true
	}
	w.HandleLuaKey(ev)
	a.RequestFrame()
	return true
}

func (c *luaCtl) registerCmd(name string, rt *luahost.Runtime) {
	if name == "" || rt == nil {
		return
	}
	if c.cmds == nil {
		c.cmds = make(map[string]*luahost.Runtime)
	}
	c.cmds[name] = rt
}

// OnCmd runs a gdbforge.register'd function: :lua name [args...]
// Pane scripts (on_key/on_tick): :lua snake [bufname] create-or-focuses that
// buffer (default via main() → open_buffer("snake"); :lua snake snake1 → new VM).
// :lua name help|-h|--help calls global help() (if any) and skips main().
func (c *luaCtl) OnCmd(args ...any) {
	a := c.app
	if len(args) == 0 {
		c.openConsole()
		return
	}
	name, _ := args[0].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		c.openConsole()
		return
	}
	if name == "console" || name == "repl" {
		c.openConsole()
		return
	}
	if name == "help" {
		topic := ""
		if len(args) > 1 {
			if s, ok := args[1].(string); ok {
				topic = strings.TrimSpace(s)
			}
		}
		c.printAPIHelp(topic)
		return
	}
	rt, err := c.ensureRuntime(name)
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
		if !c.ensureBuffer(buf, rt) && a.ctx.Log != nil {
			a.ctx.Log.Named("lua").Error("cannot open lua pane: " + buf)
		}
		a.RequestFrame()
		return
	}
	// ModeLua pane scripts (on_key/on_tick): run main() on the UI thread.
	// startJob + callOnUI deadlocks — CallNamed holds rt.mu while open_buffer
	// waits for the UI, and StartTicks/Draw need the same lock for on_tick.
	if rt.HasPaneHooks() {
		if err := rt.CallNamed(name, strArgs...); err != nil && a.ctx.Log != nil {
			a.ctx.Log.Named("lua").Error(err.Error())
		}
		a.RequestFrame()
		return
	}
	c.startJob(rt, name, strArgs)
}

// openConsole focuses the line Lua REPL (:b lua, bare :lua, :lua console).
func (c *luaCtl) openConsole() {
	a := c.app
	w := a.luaConsoleWidget
	if w == nil {
		return
	}
	c.leaveMode()
	c.ensureRepl()
	if a.Tab() != nil {
		if !a.swapFocusedWidget(w) {
			_ = a.Tab().FocusWidget(w)
		}
		a.Tab().SetInsertActive(true)
	}
	a.SetMode(platform.ModeInsert)
	w.EnsureLivePrompt()
	w.ForceFollowTailAndScroll()
	a.RequestFrame()
}

func (c *luaCtl) onReplSubmit(raw string) {
	a := c.app
	w := a.luaConsoleWidget
	if w == nil {
		return
	}
	cmd := strings.TrimSpace(raw)
	if cmd == "" {
		cmd = w.LastHistory()
		if cmd == "" {
			return
		}
	}
	w.PushHistory(cmd)
	w.EchoSubmit(cmd)
	w.ClearInput()
	w.EnsureLivePrompt()
	w.ForceFollowTailAndScroll()
	c.startReplEval(c.ensureRepl(), cmd)
}

func (c *luaCtl) onReplInterrupt() {
	a := c.app
	if w := a.luaConsoleWidget; w != nil {
		if w.Viewport() != nil && w.Viewport().HasSelection() {
			w.Viewport().CopySelection()
			return
		}
		w.ClearInput()
	}
	if !c.cancelJob() && a.ctx.Log != nil {
		a.ctx.Log.Named("lua").Info("repl interrupt")
	}
	a.RequestFrame()
}

// replGdbforgeComplete returns Tab completions for gdbforge.* in the Lua REPL line.
func (c *luaCtl) replGdbforgeComplete(text string) (string, []string) {
	rt := c.ensureRepl()
	return luahost.CompleteGdbforge(text, rt.GdbforgeMembers())
}

// printAPIHelp shows the built-in gdbforge API reference on :b io.
func (c *luaCtl) printAPIHelp(topic string) {
	a := c.app
	for _, line := range luahost.APIHelp(topic) {
		if a.outputWidget != nil {
			a.outputWidget.AppendHostLine(line)
		} else if a.ctx.Log != nil {
			a.ctx.Log.Named("lua").Info(line)
		}
	}
	a.RequestFrame()
}

func (c *luaCtl) ensureRepl() *luahost.Runtime {
	if c.repl != nil {
		return c.repl
	}
	a := c.app
	rt := luahost.New(nil, nil)
	c.wireAPI(rt)
	rt.SetPrintSink(func(line string) {
		c.replPrintLine(line)
	})
	if err := rt.LoadString(`print = function(...) gdbforge.print(...) end
help = function(...) gdbforge.help(...) end`, "@repl"); err != nil && a.ctx.Log != nil {
		a.ctx.Log.Named("lua").Error("repl print redirect: " + err.Error())
	}
	c.repl = rt
	return rt
}

func (c *luaCtl) replPrintLine(line string) {
	if line == "" {
		return
	}
	a := c.app
	c.callOnUI(func() {
		if w := a.luaConsoleWidget; w != nil {
			w.AppendOutput(line)
			w.ForceFollowTailAndScroll()
			a.RequestFrame()
		}
	})
}

func (c *luaCtl) startReplEval(rt *luahost.Runtime, line string) {
	a := c.app
	if rt == nil || line == "" {
		return
	}
	if c.jobBusy.Load() {
		if w := a.luaConsoleWidget; w != nil {
			w.AppendOutput("lua job already running — Ctrl-C to cancel")
		}
		a.RequestFrame()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.jobMu.Lock()
	c.jobCancel = cancel
	c.jobRT = rt
	c.jobBusy.Store(true)
	c.jobMu.Unlock()

	rt.SetJobContext(ctx)
	go func() {
		c.onWorker.Store(true)
		err := rt.EvalLine(line)
		c.onWorker.Store(false)

		c.jobMu.Lock()
		c.jobCancel = nil
		c.jobRT = nil
		c.jobBusy.Store(false)
		c.jobMu.Unlock()
		rt.SetJobContext(nil)

		c.callOnUI(func() {
			if w := a.luaConsoleWidget; w != nil {
				if err != nil && !errors.Is(err, luahost.ErrJobCancelled) {
					w.AppendOutput(err.Error())
				}
				w.EnsureLivePrompt()
				w.ForceFollowTailAndScroll()
			}
		})
		if scr := a.Screen(); scr != nil {
			_ = scr.PostEvent(tcell.NewEventInterrupt(luaJobDoneMsg{name: "repl", err: err}))
		}
	}()
	a.RequestFrame()
}

// startJob runs CallNamed on a worker so the UI stays responsive.
// One job at a time; Ctrl-C cancels context + kills gdbforge.system children.
func (c *luaCtl) startJob(rt *luahost.Runtime, name string, strArgs []string) {
	a := c.app
	if rt == nil || name == "" {
		return
	}
	if c.jobBusy.Load() {
		rt.AppendPrint("lua job already running — Ctrl-C to cancel")
		a.RequestFrame()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.jobMu.Lock()
	c.jobCancel = cancel
	c.jobRT = rt
	c.jobBusy.Store(true)
	c.jobMu.Unlock()

	rt.SetJobContext(ctx)
	go func() {
		c.onWorker.Store(true)
		err := rt.CallNamed(name, strArgs...)
		c.onWorker.Store(false)

		c.jobMu.Lock()
		c.jobCancel = nil
		c.jobRT = nil
		c.jobBusy.Store(false)
		c.jobMu.Unlock()
		rt.SetJobContext(nil)

		if scr := a.Screen(); scr != nil {
			_ = scr.PostEvent(tcell.NewEventInterrupt(luaJobDoneMsg{name: name, err: err}))
		}
	}()
	a.RequestFrame()
}

// JobBusy reports whether an async :lua worker job is in flight.
func (c *luaCtl) JobBusy() bool {
	return c != nil && c.jobBusy.Load()
}

// cancelJob cancels the in-flight :lua worker job. Returns true if one was active.
func (c *luaCtl) cancelJob() bool {
	c.jobMu.Lock()
	cancel := c.jobCancel
	rt := c.jobRT
	c.jobMu.Unlock()
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
// Respects job cancel so Ctrl-C cannot wedge the worker on <-done.
func (c *luaCtl) callOnUI(fn func()) {
	a := c.app
	if fn == nil {
		return
	}
	if !c.onWorker.Load() {
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
	var ctxDone <-chan struct{}
	c.jobMu.Lock()
	rt := c.jobRT
	c.jobMu.Unlock()
	if rt != nil {
		ctxDone = rt.JobContext().Done()
	}
	select {
	case <-done:
	case <-ctxDone:
	}
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

func (c *luaCtl) completions(prefix string) []string {
	fields := strings.Fields(prefix)
	trailingSpace := len(prefix) > 0 && (prefix[len(prefix)-1] == ' ' || prefix[len(prefix)-1] == '\t')

	// After a known script name, complete the next arg with help / -h / --help.
	// Sync() passes the whole rest string as prefix (e.g. "remotegdb he");
	// CmdWidget.replaceToken only replaces the last whitespace-separated token.
	if len(fields) >= 1 && c.scriptKnown(fields[0]) {
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

	return c.luaFirstTokenCompletions(prefix)
}

func (c *luaCtl) luaFirstTokenCompletions(prefix string) []string {
	fields := strings.Fields(prefix)
	token := prefix
	if len(fields) == 1 {
		token = fields[0]
	}
	names := c.scriptNameCompletions(prefix)
	add := func(name string) {
		if name == "" {
			return
		}
		if token != "" && !strings.HasPrefix(name, token) {
			return
		}
		for _, existing := range names {
			if existing == name {
				return
			}
		}
		names = append(names, name)
	}
	add("console")
	add("repl")
	add("help")
	sort.Strings(names)
	return names
}

func (c *luaCtl) scriptKnown(name string) bool {
	if name == "" {
		return false
	}
	if _, ok := c.cmds[name]; ok {
		return true
	}
	_, ok := c.pending[name]
	return ok
}

func (c *luaCtl) scriptNameCompletions(prefix string) []string {
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
	for name := range c.cmds {
		add(name)
	}
	for name := range c.pending {
		add(name)
	}
	sort.Strings(names)
	return names
}

func (c *luaCtl) maybeEnterBuffer(w interface{}) {
	lw, ok := w.(*widgets.LuaWidget)
	if !ok || lw == nil {
		return
	}
	c.enterMode(lw)
}

// loadScripts indexes :lua <basename> commands from the 3-layer search
// (project → home → embedded). VMs load lazily on first :lua / ensureRuntime.
func (c *luaCtl) loadScripts() {
	a := c.app
	files, err := luahost.ResolveLuaScripts(luacatalog.FS)
	if err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("lua").Error("resolve lua scripts: " + err.Error())
		}
		return
	}
	if c.pending == nil {
		c.pending = make(map[string]luahost.ResolvedScript)
	}
	byOrigin := map[string]int{}
	for _, f := range files {
		c.pending[f.Cmd] = f
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

// ensureRuntime returns a loaded Runtime for cmd, loading from pending on first use.
func (c *luaCtl) ensureRuntime(cmd string) (*luahost.Runtime, error) {
	a := c.app
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil, nil
	}
	if rt := c.cmds[cmd]; rt != nil {
		return rt, nil
	}
	f, ok := c.pending[cmd]
	if !ok {
		return nil, nil
	}
	rt := luahost.New(nil, c.registerCmd)
	c.wireAPI(rt)
	if err := rt.LoadScriptFile(f.Path, f.Cmd); err != nil {
		rt.Close()
		return nil, err
	}
	c.userRuntimes = append(c.userRuntimes, rt)
	c.user = rt
	delete(c.pending, cmd)
	if a.ctx.Log != nil {
		a.ctx.Log.Named("lua").Info(":lua " + f.Cmd + " loaded from " + f.Origin + " (" + f.Path + ")")
	}
	// Load may register extra names (e.g. snake_score); primary cmd is in cmds.
	if c.cmds[cmd] == nil {
		// EnsureCommand should have registered; recover if script only has main.
		_ = rt.EnsureCommand(cmd)
	}
	return c.cmds[cmd], nil
}

// wireAPI installs host callbacks shared by every user script Runtime.
func (c *luaCtl) wireAPI(rt *luahost.Runtime) {
	a := c.app
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
		if c.onWorker.Load() {
			scr := a.Screen()
			if scr == nil {
				c.openBuffer(name, rt)
				return
			}
			n := name
			_ = scr.PostEvent(tcell.NewEventInterrupt(luaUIMsg{
				fn: func() { c.openBuffer(n, rt) },
			}))
			return
		}
		c.callOnUI(func() { c.openBuffer(name, rt) })
	})
	rt.SetRun(func(argv []string) {
		c.callOnUI(func() {
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
	rt.SetForeground(func(argv []string) error {
		var err error
		c.callOnUI(func() {
			if a.TermApp == nil {
				err = fmt.Errorf("gdbforge.foreground: no screen")
				return
			}
			err = a.RunForeground(argv)
		})
		return err
	})
	rt.SetOpenExternalTTY(a.OpenExternalTTY)
	rt.SetSpawnTerminal(a.SpawnTerminal)
	rt.SetTrackChild(func(pid int) { a.children.Track(pid, false) })
	luadebug.Install(rt, luadebug.Hooks{
		SetInferiorTTY: func(path string) error {
			var err error
			c.callOnUI(func() { err = a.SetInferiorTTY(path) })
			return err
		},
		DlvConnect: func(addr string) error {
			var err error
			c.callOnUI(func() { err = a.ConnectDlv(addr) })
			return err
		},
		SpawnDlvHeadless: a.SpawnDlvHeadless,
		Program:          a.SessionProgram,
		DebuggerPath:     func() string { return a.cfg.GDBPath },
		CurrentFile: func() string {
			var path string
			c.callOnUI(func() {
				if cw := a.activeCodeWidget(); cw != nil {
					path = cw.Path()
				}
				if path == "" && a.Debug() != nil {
					path = a.Debug().CurrentFile()
				}
			})
			return path
		},
		CurrentLine: func() int {
			var line int
			c.callOnUI(func() {
				if cw := a.activeCodeWidget(); cw != nil {
					line = cw.SelLine()
				}
				if line < 1 && a.Debug() != nil {
					line = a.Debug().CurrentLine()
				}
			})
			return line
		},
		StopFile: func() string {
			var path string
			c.callOnUI(func() {
				if a.Debug() != nil {
					path = a.Debug().StopFile()
				}
				if path == "" {
					if cw := a.activeCodeWidget(); cw != nil && cw.PCLine() > 0 {
						path = cw.Path()
					}
				}
			})
			return path
		},
		StopLine: func() int {
			var line int
			c.callOnUI(func() {
				if a.Debug() != nil {
					line = a.Debug().StopLine()
				}
				if line < 1 {
					if cw := a.activeCodeWidget(); cw != nil {
						line = cw.PCLine()
					}
				}
			})
			return line
		},
		GDB: func(cmd string) {
			c.callOnUI(func() {
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
				a.console.withGdbUIOwner(func() { _ = a.backend.SendLine(sendCmd) })
				a.RequestFrame()
			})
		},
	})
	c.installSerialAPI(rt)
	c.installUartAPI(rt)
	c.installGdbAPI(rt)
}

// openBuffer focuses named panes without stealing the Code leaf via swap.
// "code" / "gdb" use leaf marks; other names use create-or-focus for pane scripts.
func (c *luaCtl) openBuffer(name string, from *luahost.Runtime) {
	a := c.app
	name = strings.TrimSpace(name)
	switch name {
	case "code":
		c.leaveMode()
		if cw := a.bufs.codeBufferForB(); cw != nil {
			a.placeCodeInSlot(cw)
		}
		a.FocusCode()
		a.RequestFrame()
	case "gdb":
		// Focus existing GDB leaf only — never relocate GDB onto the Code leaf.
		c.leaveMode()
		if a.Tab() != nil && a.gdbWidget != nil {
			if leaf := a.findGdbLeaf(); leaf != nil {
				_ = a.Tab().FocusLeaf(leaf)
			} else {
				a.Tab().FocusWidget(a.gdbWidget)
			}
			a.Tab().SetInsertActive(true)
		}
		a.SetMode(platform.ModeInsert)
		a.RequestFrame()
	case "lua":
		c.openConsole()
	default:
		a.bufs.openOrCreate(name, from)
	}
}

// ensureBuffer create-or-focuses a LuaWidget for a pane script Runtime.
// First call adopts rt; further names clone from ScriptPath() into a new VM.
func (c *luaCtl) ensureBuffer(name string, rt *luahost.Runtime) bool {
	a := c.app
	if name == "" || rt == nil || !rt.HasPaneHooks() {
		return false
	}
	if w := a.builtins[name]; w != nil {
		a.bufs.focusBufferWidget(w)
		return true
	}

	var w *widgets.LuaWidget
	if owner := c.widgetOwning(rt); owner == nil {
		w = widgets.AdoptLuaWidget(name, rt)
		c.detachUserRuntime(rt)
	} else {
		path := rt.ScriptPath()
		if path == "" {
			if a.ctx.Log != nil {
				a.ctx.Log.Named("lua").Error("cannot clone lua pane " + name + ": no script path")
			}
			return false
		}
		clone := luahost.New(nil, nil)
		c.wireAPI(clone)
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
	c.dynamic = append(c.dynamic, w)
	a.bufs.focusBufferWidget(w)
	return true
}

func (c *luaCtl) widgetOwning(rt *luahost.Runtime) *widgets.LuaWidget {
	if rt == nil {
		return nil
	}
	for _, w := range c.dynamic {
		if w != nil && w.Runtime() == rt {
			return w
		}
	}
	return nil
}

// detachUserRuntime removes rt from userRuntimes so Close won't double-free
// after the runtime is adopted by a LuaWidget.
func (c *luaCtl) detachUserRuntime(rt *luahost.Runtime) {
	if rt == nil || len(c.userRuntimes) == 0 {
		return
	}
	out := c.userRuntimes[:0]
	for _, r := range c.userRuntimes {
		if r != rt {
			out = append(out, r)
		}
	}
	c.userRuntimes = out
	if c.user == rt {
		c.user = nil
		if n := len(out); n > 0 {
			c.user = out[n-1]
		}
	}
}

// closeAll stops ticks, closes dynamic panes and user runtimes.
func (c *luaCtl) closeAll() {
	c.cancelJob()
	c.leaveMode()
	for _, w := range c.dynamic {
		if w != nil {
			w.Close()
		}
	}
	c.dynamic = nil
	for _, rt := range c.userRuntimes {
		if rt != nil {
			rt.Close()
		}
	}
	c.userRuntimes = nil
	c.user = nil
	c.pending = nil
	if c.repl != nil {
		c.repl.Close()
		c.repl = nil
	}
}
