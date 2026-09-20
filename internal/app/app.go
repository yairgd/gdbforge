package app

import (
	"strings"

	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/persist"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/termforge"
	"github.com/yairgd/termforge/commands"
	"github.com/yairgd/termforge/execcli"
	"github.com/yairgd/termforge/platform"
	"github.com/yairgd/termforge/ptyx"
)

const defaultLogFile = "gdbforge.log"

// DebuggerApp is the composition root: App loop, LayoutShell, DebugSession,
// and cross-cutting controllers (lua, search, serial, exec). Domain logic lives
// on embedded DebugSession controllers and LayoutShell policy.
type DebuggerApp struct {
	*termforge.App
	LayoutShell
	DebugSession

	commandReg     *commands.CommandRegistry
	keyBindings    *commands.KeyBindingRegistry
	insertKeys     *commands.KeyBindingRegistry
	completionKeys *commands.KeyBindingRegistry

	cmdWidget *termforge.CmdWidget
	compPopup *termforge.CompletionPopupWidget
	ctx       platform.AppContext
	cfg       SessionConfig
	version   string
	fileLog   *platform.FileSink

	comp            completionCtl
	cmd             cmdCtl
	search          searchCtl
	lua             luaCtl
	serial          serialCtl
	children        termforge.ChildProcCtl
	extInferiorHold *externalInferiorHold

	execClient *execcli.ExecClient
	execWidget *widgets.ExecWidget

	builtins    map[string]termforge.NodeWidget
	aboutWidget *widgets.AboutWidget
	helpWidget  *widgets.HelpWidget
	logoWidget  *widgets.LogoWidget
}

// NewDebuggerApp builds the composition root. version is the link-time stamp
// from package main (":b about" shows "not for release" for "dev").
func NewDebuggerApp(cfg SessionConfig, version string) (*DebuggerApp, error) {
	dbg := &DebuggerApp{cfg: cfg, version: version}
	dbg.initControllers()
	// Start the debugger before tcell Init() so a missing gdb/dlv binary
	// exits on the normal terminal (cgdb-style) instead of leaving alt-screen on.
	if err := dbg.initBackend(); err != nil {
		return nil, err
	}
	dbg.App = termforge.NewApp()
	dbg.App.Api = dbg
	dbg.commandReg = commands.NewCommandRegistry()
	if err := dbg.InitB(); err != nil {
		dbg.Close()
		return nil, err
	}
	return dbg, nil
}

// GDB returns the owned debugger session for external APIs (e.g. MCP).
func (a *DebuggerApp) GDB() ptyx.Session {
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

func (a *DebuggerApp) Close() {
	a.closeExternalInferiorHold()
	a.lua.closeAll()
	a.DebugSession.close(a)
	a.saveCmdlineHistoryOnQuit()
	a.serial.Close()
	a.children.KillAll()
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
	if a.App != nil {
		a.App.Close()
	}
}

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

func (a *DebuggerApp) saveCmdlineHistoryOnQuit() {
	if a == nil || a.cmdWidget == nil {
		return
	}
	_ = persist.SaveCmdlineHistory(".", a.cmdWidget.CommandHistoryItems(), a.cmdWidget.SearchHistoryItems())
}

func (a *DebuggerApp) Debug() *debugstate.State {
	if a == nil {
		return nil
	}
	return a.debug
}
