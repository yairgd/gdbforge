package main

import (
	"sync"

	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/execcli"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
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
	commandReg     *commands.CommandRegistry
	keyBindings    *commands.KeyBindingRegistry
	insertKeys     *commands.KeyBindingRegistry
	completionKeys *commands.KeyBindingRegistry

	tab           *termui.TabWidget
	cmdWidget     *termui.CmdWidget
	completionMenu *termui.CompletionMenu
	completionView termui.CompletionView
	completionBar  *termui.CompletionBarWidget // concrete chrome; also CompletionView
	ctx            platform.AppContext
	miLog          *platform.NamedLogger

	cfg              SessionConfig
	gdbClient        *gdb.GDBClient
	gdbCancelSub     func()
	inferiorCancelSub func()
	gdbInputState    *gdb.GdbInputState
	pendingFrameSync bool
	pendingDebugInfo bool // refresh threads/stack after *stopped once (gdb) is ready
	gdbWidget        *widgets.GDBWidget
	gdbMcp           *mcp.GdbMcpService

	execClient *execcli.ExecClient
	execWidget *widgets.ExecWidget

	// widgetJump is a Vim-style jump list of prior widgets in the focused pane
	// (pushed on :b / :e / :! swaps; Ctrl-O pops).
	widgetJump []termui.Widget

	// builtins are singleton views created once at startup (:b about, :b gdb, …).
	builtins    map[string]termui.Widget
	aboutWidget *widgets.AboutWidget
	helpWidget  *widgets.HelpWidget
	logoWidget  *widgets.LogoWidget

	// fileBuffers are per-path CodeWidgets opened via :e / GDB stop (PaneName = basename).
	fileBuffers map[string]*widgets.CodeWidget
	// bufferListed paths appear in :b Tab. Only :edit / FileList open marks them —
	// stop / callstack / BP preview must not pollute the wildmenu (ldo.c, …).
	bufferListed map[string]struct{}

	breakpoints      *models.BreakpointList
	threads          *models.ThreadList
	callstack        *models.CallStack
	bpWidget         *widgets.BreakpointWidget
	threadWidget     *widgets.ThreadWidget
	callstackWidget  *widgets.CallStackWidget
	outputWidget     *widgets.OutputWidget
	fileListWidget   *widgets.FileListWidget
	primaryCode      *widgets.CodeWidget
	bpRefreshMu      sync.Mutex
	bpRefreshRunning bool
	bpRefreshPending bool
	debugInfoMu      sync.Mutex
	debugInfoRunning bool
	debugInfoPending bool

	// completionForGDB is true while ModeCompletion is driven by GDB Tab
	// (apply/cancel return to insert mode instead of command mode).
	completionForGDB bool
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
	if a == nil {
		return nil
	}
	return a.gdbClient
}

// Close tears down owned debugger/exec sessions.
func (a *DebuggerApp) Close() {
	if a.gdbMcp != nil {
		a.gdbMcp.Close()
		a.gdbMcp = nil
	}
	if a.inferiorCancelSub != nil {
		a.inferiorCancelSub()
		a.inferiorCancelSub = nil
	}
	if a.gdbCancelSub != nil {
		a.gdbCancelSub()
		a.gdbCancelSub = nil
	}
	if a.gdbClient != nil {
		a.gdbClient.Close()
		a.gdbClient = nil
	}
	if a.execClient != nil {
		a.execClient.Close()
		a.execClient = nil
	}
}
