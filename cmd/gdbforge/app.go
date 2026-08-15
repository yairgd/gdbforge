package main

import (
	"strings"

	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/execcli"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/persist"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

const defaultLogFile = "gdbforge.log"

const (
	cmdBreak termui.CommandID = iota + 2
	cmdContinue
	cmdNext
	cmdStep
	cmdPrint
	cmdBacktrace
	cmdInfo
	cmdRun
)

type DebuggerApp struct {
	*termui.TermApp
	commandReg     *commands.CommandRegistry
	keyBindings    *commands.KeyBindingRegistry
	insertKeys     *commands.KeyBindingRegistry
	completionKeys *commands.KeyBindingRegistry

	ws        *Workspace
	cmdWidget *termui.CmdWidget
	ctx       platform.AppContext
	debug     *debugstate.State
	miLog     *platform.NamedLogger
	fileLog   *platform.FileSink // optional; -log / --log or :set log

	// Controllers own their domain state and behavior; DebuggerApp wires them
	// (initControllers) and keeps orchestration.
	cfg        SessionConfig
	backend    backend.Backend
	breaks     breakCtl
	asm        asmCtl
	bufs       bufferCtl
	debugInfo  debugInfoCtl
	console    consoleCtl
	inferiorIO inferiorIOCtl
	comp       completionCtl
	search     searchCtl
	lua        luaCtl
	dlv        dlvCtl

	gdbWidget *widgets.GDBWidget
	gdbMcp    *mcp.GdbMcpService

	execClient *execcli.ExecClient
	execWidget *widgets.ExecWidget

	// builtins are singleton views created once at startup (:b about, :b gdb, …).
	builtins    map[string]termui.Widget
	aboutWidget *widgets.AboutWidget
	helpWidget  *widgets.HelpWidget
	logoWidget  *widgets.LogoWidget

	bpWidget       *widgets.BreakpointWidget
	outputWidget   *widgets.OutputWidget
	luaConsoleWidget *widgets.LuaConsoleWidget
	fileListWidget *widgets.FileListWidget
}

func NewDebuggerApp(cfg SessionConfig) (*DebuggerApp, error) {
	dbg := &DebuggerApp{cfg: cfg}
	dbg.initControllers()
	dbg.TermApp = termui.NewTermApp()
	dbg.TermApp.Api = dbg
	dbg.commandReg = commands.NewCommandRegistry()
	if err := dbg.InitB(); err != nil {
		dbg.Close()
		return nil, err
	}
	dbg.HandleResize()
	return dbg, nil
}

// GDB returns the owned debugger session for external APIs (e.g. MCP).
// Despite the name, this is whichever backend was selected with -g (gdb or dlv).
func (a *DebuggerApp) GDB() core.Session {
	if a == nil || a.backend == nil {
		return nil
	}
	return a.backend.Session()
}

func (a *DebuggerApp) isDLV() bool {
	return a != nil && a.backend != nil && a.backend.Kind() == backend.DLV
}

func (a *DebuggerApp) gdbBackend() *backend.GDBBackend {
	if a == nil || a.backend == nil {
		return nil
	}
	b, _ := a.backend.(*backend.GDBBackend)
	return b
}

func (a *DebuggerApp) dlvBackend() *backend.DLVBackend {
	if a == nil || a.backend == nil {
		return nil
	}
	b, _ := a.backend.(*backend.DLVBackend)
	return b
}

// enableFileLog starts (or switches) append logging to path.
// Empty path means defaultLogFile ("gdbforge.log").
func (a *DebuggerApp) enableFileLog(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultLogFile
	}
	if a.fileLog != nil && a.cfg.LogFile == path {
		return nil
	}
	if a.fileLog != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.RemoveSink(a.fileLog)
		}
		_ = a.fileLog.Close()
		a.fileLog = nil
	}
	sink, err := platform.NewFileSink(path)
	if err != nil {
		return err
	}
	a.fileLog = sink
	a.cfg.LogFile = path
	if a.ctx.Log != nil {
		a.ctx.Log.AddSink(sink)
	}
	return nil
}

// Close tears down owned debugger/exec sessions.
func (a *DebuggerApp) Close() {
	a.lua.closeAll()
	a.breaks.saveOnQuit()
	a.saveCmdlineHistoryOnQuit()
	if a.gdbMcp != nil {
		a.gdbMcp.Close()
		a.gdbMcp = nil
	}
	a.inferiorIO.stop()
	a.console.stopBridge()
	if a.backend != nil {
		a.backend.Close()
		a.backend = nil
	}
	if a.execClient != nil {
		a.execClient.Close()
		a.execClient = nil
	}
	if a.fileLog != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.RemoveSink(a.fileLog)
		}
		_ = a.fileLog.Close()
		a.fileLog = nil
	}
}

// restoreCmdlineHistory loads ./.gdbforge/cmdline_history.yaml into the CmdWidget.
func (a *DebuggerApp) restoreCmdlineHistory() {
	if a == nil || a.cmdWidget == nil {
		return
	}
	cmds, search, err := persist.LoadCmdlineHistory(".")
	if err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("cmdline").Error("load cmdline history: " + err.Error())
		}
		return
	}
	a.cmdWidget.LoadCommandHistory(cmds)
	a.cmdWidget.LoadSearchHistory(search)
}

// saveCmdlineHistoryOnQuit writes CmdWidget history to ./.gdbforge/cmdline_history.yaml.
func (a *DebuggerApp) saveCmdlineHistoryOnQuit() {
	if a == nil || a.cmdWidget == nil {
		return
	}
	_ = persist.SaveCmdlineHistory(".", a.cmdWidget.CommandHistoryItems(), a.cmdWidget.SearchHistoryItems())
}

// Debug returns gdbforge-private debugger session state.
func (a *DebuggerApp) Debug() *debugstate.State {
	if a == nil {
		return nil
	}
	return a.debug
}
