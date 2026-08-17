package main

import (
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
	"github.com/yairgd/gdbforge/internal/termui"

	tcell "github.com/gdamore/tcell/v2"
)

// breakHost is the narrow surface breakCtl needs from the composition root.
// DebuggerApp implements it; breakCtl must not depend on *DebuggerApp.
type breakHost interface {
	Backend() backend.Backend
	Session() core.Session
	State() *platform.AppState
	Debug() *debugstate.State
	BPWidget() *widgets.BreakpointWidget
	AssemblyWidget() *widgets.AssemblyWidget
	FileBuffers() map[string]*widgets.CodeWidget
	PrimaryCode() *widgets.CodeWidget
	GDBWidget() *widgets.GDBWidget
	Screen() tcell.Screen
	PublishBreakpointsChanged()
	DeferDLVBPRefresh()
	IsDLVConfirming() bool
	GdbMcp() *mcp.GdbMcpService
}

// breakCtl owns breakpoint domain logic and BP model state.
// DebuggerApp wires callbacks into it; ActivateBreakpoint stays on DebuggerApp.
type breakCtl struct {
	host        breakHost
	list        *models.BreakpointList
	snapshot    []models.BreakInfo
	snapshotSet bool
	coalesce    coalesceRunner
}

// searchCtl owns '/' / n/N / */# SearchHost policy and the last search target.
// Mode entry/exit and search-vs-GDB-next stay on *DebuggerApp (orchestration).
type searchCtl struct {
	app    *DebuggerApp
	target termui.SearchHost
}

// luaCtl is declared in lua.go (owns scripting state and domain methods).
// dlvCtl is declared in dlv_ctl.go (owns Delve confirm + stop/frame sync).
// asmCtl, bufferCtl, debugInfoCtl, consoleCtl, inferiorIOCtl and completionCtl
// are declared next to their domain code (assembly.go, buffers.go, …).

// initControllers points every controller at the composition root. DebuggerApp
// only wires; each ctl owns its own domain state and behavior.
func (a *DebuggerApp) initControllers() {
	a.breaks.host = a
	a.asm.host = a
	a.bufs.host = a
	a.debugInfo.host = a
	a.console.host = a
	a.inferiorIO.host = a
	a.comp.host = a
	a.search.app = a
	a.lua.app = a
	a.dlv.app = a
}

// --- Composition-root adapters (host interfaces) ---
//
// Controller hosts reuse the DebuggerApp helpers they need; the adapters below
// only add the surface a ctl cannot reach on its own (peer controllers, owned
// widgets, and bus publishing).

func (a *DebuggerApp) Backend() backend.Backend { return a.backend }
func (a *DebuggerApp) Session() core.Session    { return a.GDB() }

// Tab returns the generic TabWidget owned by Workspace (tree ops only).
// Workspace policy lives on Workspace / DebuggerApp delegates, not here.
func (a *DebuggerApp) Tab() *termui.TabWidget {
	if a == nil || a.ws == nil {
		return nil
	}
	return a.ws.Tab()
}

// Workspace returns the gdbforge workspace policy layer above TabWidget.
func (a *DebuggerApp) Workspace() *Workspace {
	if a == nil {
		return nil
	}
	return a.ws
}

func (a *DebuggerApp) BPWidget() *widgets.BreakpointWidget {
	return a.bpWidget
}
func (a *DebuggerApp) AssemblyWidget() *widgets.AssemblyWidget {
	return a.asm.Widget()
}
func (a *DebuggerApp) FileBuffers() map[string]*widgets.CodeWidget {
	return a.bufs.Buffers()
}
func (a *DebuggerApp) PrimaryCode() *widgets.CodeWidget    { return a.bufs.Primary() }
func (a *DebuggerApp) CodeBufferForB() *widgets.CodeWidget { return a.bufs.codeBufferForB() }
func (a *DebuggerApp) GDBWidget() *widgets.GDBWidget       { return a.gdbWidget }
func (a *DebuggerApp) LuaConsoleWidget() *widgets.LuaConsoleWidget {
	return a.luaConsoleWidget
}
func (a *DebuggerApp) LuaGdbforgeComplete(text string) (string, []string) {
	return a.lua.replGdbforgeComplete(text)
}
func (a *DebuggerApp) GdbMcp() *mcp.GdbMcpService          { return a.gdbMcp }
func (a *DebuggerApp) CmdWidget() *termui.CmdWidget        { return a.cmdWidget }
func (a *DebuggerApp) LogoWidget() *widgets.LogoWidget     { return a.logoWidget }
func (a *DebuggerApp) OutputWidget() *widgets.OutputWidget { return a.outputWidget }
func (a *DebuggerApp) FileListWidget() *widgets.FileListWidget {
	return a.fileListWidget
}
func (a *DebuggerApp) Builtins() map[string]termui.Widget { return a.builtins }

// LogError writes a controller-side error to the named log area (nil-safe).
func (a *DebuggerApp) LogError(area, msg string) {
	if a == nil || a.ctx.Log == nil {
		return
	}
	a.ctx.Log.Named(area).Error(msg)
}

func (a *DebuggerApp) PublishBreakpointsChanged() {
	if a == nil || a.ctx.Bus == nil {
		return
	}
	platform.Publish(a.ctx.Bus, BreakpointsChangedMsg{})
}

func (a *DebuggerApp) PublishCompletion(msg termui.CompletionMsg) {
	if a == nil || a.ctx.Bus == nil {
		return
	}
	platform.Publish(a.ctx.Bus, msg)
}

// --- breakCtl peers ---

func (a *DebuggerApp) BreakpointsChanged() { a.breaks.onChanged() }
func (a *DebuggerApp) PaintAsmBreaks()     { a.breaks.paintAsmMarks(a.breaks.Items()) }
func (a *DebuggerApp) PaintCodeBreaks(w *widgets.CodeWidget, path string) {
	a.breaks.paintCodeWidget(w, path)
}
func (a *DebuggerApp) ToggleCodeBreak(path string, line int) {
	a.breaks.onCodeBreakToggle(path, line)
}

// --- asmCtl peers ---

func (a *DebuggerApp) OpenAssembly()       { a.asm.openBuffer() }
func (a *DebuggerApp) SetPreferAsm(v bool) { a.asm.setPreferAsm(v) }
func (a *DebuggerApp) IsDLV() bool         { return a.isDLV() }

// --- bufferCtl peers ---

func (a *DebuggerApp) ShowCodeAt(file string, line int) *widgets.CodeWidget {
	return a.bufs.showCodeAt(file, line)
}

// --- debugInfoCtl peers ---

func (a *DebuggerApp) SelectedFrameLevel() int { return a.debugInfo.selectedLevel() }

// --- dlvCtl peers ---

func (a *DebuggerApp) DeferDLVBPRefresh() { a.dlv.deferBPRefresh() }
func (a *DebuggerApp) TakeDeferredBP() bool {
	return a.dlv.takeDeferredBP()
}
func (a *DebuggerApp) IsDLVConfirming() bool {
	return a.isDLV() && a.dlv.confirm.Confirming()
}
func (a *DebuggerApp) NoteStackNavGDB() { a.dlv.noteStackNavGDB() }
func (a *DebuggerApp) NoteStackNavDLV(cmd string, curLevel int) {
	a.dlv.noteStackNavDLV(cmd, curLevel)
}
func (a *DebuggerApp) DlvConfirming() bool             { return a.dlv.confirm.Confirming() }
func (a *DebuggerApp) DlvObserveUpdate(upd dlv.Update) { a.dlv.confirm.Observe(upd) }
func (a *DebuggerApp) DlvConfirmHost() string          { return a.dlv.confirm.Host() }
func (a *DebuggerApp) BumpCodeNav()                    { a.dlv.bumpCodeNav() }
func (a *DebuggerApp) SuppressDlvStopUI()              { a.dlv.suppressStopUI++ }
func (a *DebuggerApp) SuppressStopUICount() int        { return a.dlv.suppressStopUI }
func (a *DebuggerApp) ClearSuppressStopUI()            { a.dlv.clearSuppressStopUI() }
func (a *DebuggerApp) ApplyPendingFrameSync(promptReady, isError bool) bool {
	return a.dlv.applyPendingFrameSync(promptReady, isError)
}

func (a *DebuggerApp) MaybeEnableRemoteMode(cmd string) {
	a.maybeEnableRemoteMode(cmd)
}

// TriggerPendingDebugInfoIfReady runs a post-stop threads/stack refresh once the
// debugger prompt is back (dlvCtl drops the request when nothing is armed).
func (a *DebuggerApp) TriggerPendingDebugInfoIfReady(promptReady bool) {
	if !promptReady {
		return
	}
	a.dlv.triggerPendingDebugInfo()
}

func (a *DebuggerApp) TriggerPendingStackRefreshIfReady(promptReady bool) {
	if !promptReady {
		return
	}
	a.dlv.triggerPendingStackRefresh()
}

// --- consoleCtl / inferiorIOCtl / luaCtl peers ---

func (a *DebuggerApp) ConsoleSuspend() { a.console.onGdbConsoleSuspend() }

func (a *DebuggerApp) sendInferior(tty *ptyx.TTY, send func()) { a.inferiorIO.send(tty, send) }

func (a *DebuggerApp) LuaEnterBuffer(w termui.Widget) { a.lua.maybeEnterBuffer(w) }
func (a *DebuggerApp) LuaEnsureBuffer(name string, from *luahost.Runtime) bool {
	return a.lua.ensureBuffer(name, from)
}

var (
	_ breakHost      = (*DebuggerApp)(nil)
	_ asmHost        = (*DebuggerApp)(nil)
	_ bufferHost     = (*DebuggerApp)(nil)
	_ debugInfoHost  = (*DebuggerApp)(nil)
	_ consoleHost    = (*DebuggerApp)(nil)
	_ inferiorHost   = (*DebuggerApp)(nil)
	_ completionHost = (*DebuggerApp)(nil)
)
