package main

import (
	"sync"

	"sync/atomic"

	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/execcli"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/persist"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
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

	tab            *termui.TabWidget
	cmdWidget      *termui.CmdWidget
	completionMenu *termui.CompletionMenu
	completionView termui.CompletionView
	completionBar  *termui.CompletionBarWidget // concrete chrome; also CompletionView
	ctx            platform.AppContext
	debug          *debugstate.State
	miLog          *platform.NamedLogger

	cfg               SessionConfig
	gdbClient         *gdb.GDBClient
	dlvClient         *dlv.Client
	gdbCancelSub      func()
	inferiorCancelSub func()
	// gdbBridgeGen identifies the active debugger console bridge. Bump before
	// canceling a subscription so a deliberate restart does not post gdb-exit.
	gdbBridgeGen  atomic.Uint64
	gdbInputState *gdb.GdbInputState
	dlvInputState *dlv.InputState
	// dlvConfirm tracks Delve [Y/n]? prompts (suspended BP after exit, etc.).
	dlvConfirm dlv.ConfirmGate
	// dlvBPDeferred is set when a BP refresh was skipped while dlvConfirm is active.
	dlvBPDeferred        bool
	pendingFrameSync     bool
	pendingFrameLevel    int // Delve: level to show after frame/up/down (see pendingFrameLevelSet)
	pendingFrameLevelSet bool
	pendingDebugInfo     bool // refresh threads/stack after *stopped once prompt is ready
	// codeNavGen increments when the user browses away from the stop frame
	// (call stack / frame cmd). Late stop refreshes with an older gen are ignored.
	codeNavGen uint64
	// dlvSuppressStopUI counts Delve frame/up/down ops whose re-emitted "> …"
	// lines must not run stop UI (would snap Code back to frame 0).
	dlvSuppressStopUI int
	gdbWidget         *widgets.GDBWidget
	gdbMcp            *mcp.GdbMcpService

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

	breakpoints *models.BreakpointList
	// bpSnapshot is the last user-visible BP list for quit save. Kept across
	// clearBreakpointViews (kill/exit UI reset) so q / Ctrl-D can still persist.
	bpSnapshot       []models.BreakInfo
	bpSnapshotSet    bool
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

	// Lua / gdbforge scripting
	luaScratch      *widgets.LuaWidget
	luaSnake        *widgets.LuaWidget
	luaTetris       *widgets.LuaWidget
	activeLua       *widgets.LuaWidget
	luaCmds         map[string]*luahost.Runtime
	luaUser         *luahost.Runtime
	luaUserRuntimes []*luahost.Runtime
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

// GDB returns the owned debugger session for external APIs (e.g. MCP).
// Despite the name, this is whichever backend was selected with -g (gdb or dlv).
func (a *DebuggerApp) GDB() core.Session {
	if a == nil {
		return nil
	}
	if a.dlvClient != nil {
		return a.dlvClient
	}
	return a.gdbClient
}

func (a *DebuggerApp) isDLV() bool {
	return a != nil && a.cfg.IsDLV()
}

// Close tears down owned debugger/exec sessions.
func (a *DebuggerApp) Close() {
	a.saveBreakpointsOnQuit()
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
	if a.dlvClient != nil {
		a.dlvClient.Close()
		a.dlvClient = nil
	}
	if a.gdbClient != nil {
		a.gdbClient.Close()
		a.gdbClient = nil
	}
	if a.execClient != nil {
		a.execClient.Close()
		a.execClient = nil
	}
	a.leaveLuaMode()
	for _, rt := range a.luaUserRuntimes {
		if rt != nil {
			rt.Close()
		}
	}
	a.luaUserRuntimes = nil
	a.luaUser = nil
	for _, w := range []*widgets.LuaWidget{a.luaScratch, a.luaSnake, a.luaTetris} {
		if w != nil {
			w.Close()
		}
	}
}

// saveBreakpointsOnQuit writes bpSnapshot to ./.gdbforge/breakpoints.yaml.
func (a *DebuggerApp) saveBreakpointsOnQuit() {
	if a == nil || !a.bpSnapshotSet {
		return
	}
	// Prefer live model when still populated; else last snapshot before UI clear.
	items := a.bpSnapshot
	if a.breakpoints != nil {
		if cur := a.breakpoints.Items(); len(cur) > 0 {
			items = cur
		}
	}
	_ = persist.SaveBreakpoints(".", items)
}

// Debug returns gdbforge-private debugger session state.
func (a *DebuggerApp) Debug() *debugstate.State {
	if a == nil {
		return nil
	}
	return a.debug
}
