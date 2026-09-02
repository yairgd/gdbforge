package main

import (
	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/persist"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

// DebugSession owns the debugger backend, shared debug models, and GDB/DLV
// controllers (console bridge, breakpoints, threads/stack, stop pipeline).
// DebuggerApp embeds it and wires host interfaces at initControllers.
type DebugSession struct {
	backend    backend.Backend
	debug      *debugstate.State
	miLog      *platform.NamedLogger
	gdbWidget  *widgets.GDBWidget
	gdbMcp     *mcp.GdbMcpService
	breaks     breakCtl
	asm        asmCtl
	bufs       bufferCtl
	debugInfo  debugInfoCtl
	console    consoleCtl
	inferiorIO inferiorIOCtl
	dlv        dlvCtl
	bpWidget   *widgets.BreakpointWidget
	outputWidget *widgets.OutputWidget
	luaConsoleWidget *widgets.LuaConsoleWidget
	fileListWidget   *widgets.FileListWidget
}

func (s *DebugSession) init(a *DebuggerApp) error {
	if s == nil || a == nil {
		return nil
	}
	s.bufs.initMaps()

	var (
		boot          string
		inferior      *ptyx.TTY
		skipBreakMain bool
		extTTY        = inferiorTTYFromEnvOrCfg(a.cfg)
	)
	if s.backend == nil {
		if a.cfg.IsDLV() {
			client, err := dlv.NewClientOpts(a.cfg.GDBPath, a.cfg.GDBArgs, dlv.ClientOptions{InferiorTTY: extTTY})
			if err != nil {
				return err
			}
			s.backend = backend.NewDLV(client)
		} else {
			client, err := gdb.NewGDBClientOpts(a.cfg.GDBPath, a.cfg.GDBArgs, gdb.ClientOptions{InferiorTTY: extTTY})
			if err != nil {
				return err
			}
			s.backend = backend.NewGDB(client)
		}
	}
	if !a.cfg.IsDLV() {
		skipBreakMain = gdb.HasInitScript(a.cfg.GDBArgs)
	}
	boot = s.backend.TakeStartupOutput()
	inferior = s.backend.InferiorTTY()
	_ = s.backend.ConfigureInferiorTTY()

	if s.backend.PaintTargetInConsole() {
		s.debug.SetGdbTargetPrint(true)
	}

	s.gdbWidget = widgets.NewGDBWidget()
	s.gdbWidget.SetClipboard(a.ClipboardIO())
	if cli := debuggerCLITTY(s.backend); cli != nil {
		s.console.wireCLI(s.gdbWidget, cli, a.RequestFrame)
	}
	if boot != "" {
		s.gdbWidget.WriteBoot(boot)
	}
	s.console.startGdbConsoleBridge()
	a.registerBuiltin("gdb", s.gdbWidget)

	s.outputWidget = widgets.NewOutputWidget()
	s.outputWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("io", s.outputWidget)
	a.registerBuiltin("output", s.outputWidget)
	a.maybeClearOutput()
	if extTTY != "" {
		s.inferiorIO.markExternal(extTTY)
	} else if inferior != nil {
		s.inferiorIO.wire(inferior)
	}

	savedBPs, err := persist.LoadBreakpoints(".")
	if err != nil && a.ctx.Log != nil {
		a.ctx.Log.Named("breakpoints").Error("load saved breakpoints: " + err.Error())
	}
	if !skipBreakMain && len(savedBPs) == 0 {
		s.breaks.maybeBreakMain()
	}

	s.breaks.list = &models.BreakpointList{}
	s.debugInfo.threads = &models.ThreadList{}
	s.debugInfo.stack = &models.CallStack{}
	s.asm.list = &models.AssemblyList{}
	s.asm.widget = widgets.NewAssemblyWidget()
	s.asm.widget.Ctx = a.ctx
	s.asm.widget.SetClipboard(a.ClipboardIO())
	s.asm.widget.SetAppState(s.debug)
	a.registerBuiltin("asm", s.asm.widget)
	a.registerBuiltin("assembly", s.asm.widget)

	s.bpWidget = widgets.NewBreakpointWidget()
	s.bpWidget.Ctx = a.ctx
	s.bpWidget.SetClipboard(a.ClipboardIO())
	s.bpWidget.SetAppState(s.debug)
	a.registerBuiltin("breakpoint", s.bpWidget)

	s.debugInfo.threadW = widgets.NewThreadWidget()
	s.debugInfo.threadW.Ctx = a.ctx
	s.debugInfo.threadW.SetClipboard(a.ClipboardIO())
	s.debugInfo.threadW.SetAppState(s.debug)
	a.registerBuiltin("threads", s.debugInfo.threadW)

	s.debugInfo.stackW = widgets.NewCallStackWidget()
	s.debugInfo.stackW.Ctx = a.ctx
	s.debugInfo.stackW.SetClipboard(a.ClipboardIO())
	s.debugInfo.stackW.SetAppState(s.debug)
	a.registerBuiltin("callstack", s.debugInfo.stackW)

	s.fileListWidget = widgets.NewFileListWidget()
	s.fileListWidget.Ctx = a.ctx
	s.fileListWidget.SetClipboard(a.ClipboardIO())
	s.fileListWidget.SetAppState(s.debug)

	s.luaConsoleWidget = widgets.NewLuaConsoleWidget()
	s.luaConsoleWidget.SetClipboard(a.ClipboardIO())
	s.luaConsoleWidget.WireConsole(&widgets.ConsoleHandlers{
		Submit:    a.lua.onReplSubmit,
		Interrupt: a.lua.onReplInterrupt,
	})
	a.registerBuiltin("lua", s.luaConsoleWidget)

	a.lua.cmds = make(map[string]*luahost.Runtime)
	a.lua.loadScripts()

	a.registerLayouts()

	s.gdbMcp = mcp.NewGdbMcpService(a.GDB(), a.State())
	s.gdbMcp.OnBreakpointsChanged = s.breaks.onChanged
	s.gdbMcp.SetDomain(appDebugDomain{app: a})
	if a.cfg.IsDLV() {
		s.gdbMcp.SetPromptToken(dlv.PromptToken)
	}
	s.breaks.restoreSaved(savedBPs)
	return nil
}

func debuggerCLITTY(b backend.Backend) *ptyx.TTY {
	if b == nil {
		return nil
	}
	switch b.Kind() {
	case backend.GDB:
		if gb, ok := b.(*backend.GDBBackend); ok && gb.Client != nil {
			return gb.Client.CLITTY()
		}
	case backend.DLV:
		if db, ok := b.(*backend.DLVBackend); ok && db.Client != nil {
			return db.Client.TTY
		}
	}
	return nil
}

func (s *DebugSession) close(a *DebuggerApp) {
	if s == nil {
		return
	}
	s.breaks.saveOnQuit()
	if s.gdbMcp != nil {
		s.gdbMcp.Close()
		s.gdbMcp = nil
	}
	s.inferiorIO.unwire()
	s.console.stopBridge()
	if s.backend != nil {
		s.backend.Close()
		s.backend = nil
	}
}
