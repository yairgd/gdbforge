package main

import (
	"sync"

	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/commands"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/execcli"
	"github.com/yairgd/cgdb-go/internal/mcp"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

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
	commandReg  *commands.CommandRegistry
	keyBindings *commands.KeyBindingRegistry

	tab            *termui.TabWidget
	cmdWidget      *termui.CmdWidget
	completionBar  *termui.CompletionBarWidget
	ctx            platform.AppContext
	miLog          *platform.NamedLogger

	cfg       SessionConfig
	gdbWidget *widgets.GDBWidget
	gdbMcp    *mcp.GdbMcpService

	execClient *execcli.ExecClient
	execWidget *widgets.ExecWidget

	// widgetJump is a Vim-style jump list of prior widgets in the focused pane
	// (pushed on :b / :e / :! swaps; Ctrl-O pops).
	widgetJump []termui.Widget

	// builtins are singleton views created once at startup (:b about, :b gdb, …).
	builtins    map[string]termui.Widget
	aboutWidget *widgets.AboutWidget

	// fileBuffers are per-path CodeWidgets opened via :e / GDB stop (PaneName = basename).
	fileBuffers map[string]*widgets.CodeWidget

	bpWidget           *widgets.BreakpointWidget
	threadWidget       *widgets.ThreadWidget
	callstackWidget    *widgets.CallStackWidget
	outputWidget       *widgets.OutputWidget
	fileListWidget     *widgets.FileListWidget
	primaryCode        *widgets.CodeWidget
	bpRefreshMu        sync.Mutex
	bpRefreshRunning   bool
	bpRefreshPending   bool
	debugInfoMu        sync.Mutex
	debugInfoRunning   bool
	debugInfoPending   bool
}

func NewDebuggerApp(cfg SessionConfig) (*DebuggerApp, error) {
	dbg := &DebuggerApp{cfg: cfg}
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

// GDB returns the owned GDB session for external APIs (e.g. MCP).
func (a *DebuggerApp) GDB() core.Session {
	if a.gdbWidget == nil {
		return nil
	}
	return a.gdbWidget.Session()
}

// Close tears down owned debugger/exec sessions.
func (a *DebuggerApp) Close() {
	if a.gdbMcp != nil {
		a.gdbMcp.Close()
		a.gdbMcp = nil
	}
	if a.gdbWidget != nil {
		a.gdbWidget.Close()
	}
	if a.execClient != nil {
		a.execClient.Close()
		a.execClient = nil
	}
}
