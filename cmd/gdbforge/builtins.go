package main

import (
	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/persist"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
	"github.com/yairgd/gdbforge/internal/termui"
)

// initBuiltins creates singleton built-in views once at startup.
// Adding a new page: construct it here, registerBuiltin(name, w).
// Show with :b name (OnBuffer). Source files use :edit filename (per-file CodeWidget).
func (a *DebuggerApp) initBuiltins() error {
	a.builtins = make(map[string]termui.Widget)
	a.fileBuffers = make(map[string]*widgets.CodeWidget)
	a.bufferListed = make(map[string]struct{})

	a.aboutWidget = widgets.NewAboutWidget(version)
	a.registerBuiltin("about", a.aboutWidget)

	a.helpWidget = widgets.NewHelpWidget()
	a.helpWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("help", a.helpWidget)

	a.logoWidget = widgets.NewLogoWidget()

	logWidget := termui.NewLoggerWidget(a.ctx)
	logWidget.Events = a.Events()
	logWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("logger", logWidget)

	var (
		boot          string
		inferior      *ptyx.TTY
		promptTok     string
		skipBreakMain bool
		extTTY        = inferiorTTYFromEnvOrCfg(a.cfg)
	)
	if a.cfg.IsDLV() {
		client, err := dlv.NewClientOpts(a.cfg.GDBPath, a.cfg.GDBArgs, dlv.ClientOptions{InferiorTTY: extTTY})
		if err != nil {
			return err
		}
		a.dlvClient = client
		a.dlvInputState = dlv.NewInputState()
		boot = client.TakeStartupOutput()
		inferior = client.InferiorTTY()
		promptTok = dlv.PromptToken
		_ = client.ConfigureInferiorTTY()
	} else {
		client, err := gdb.NewGDBClientOpts(a.cfg.GDBPath, a.cfg.GDBArgs, gdb.ClientOptions{InferiorTTY: extTTY})
		if err != nil {
			return err
		}
		a.gdbClient = client
		a.gdbInputState = gdb.NewGdbInputState()
		boot = client.TakeStartupOutput()
		inferior = client.InferiorTTY()
		promptTok = gdb.MIPromptToken
		_ = client.ConfigureInferiorTTY()
		skipBreakMain = gdb.HasInitScript(a.cfg.GDBArgs)
	}

	a.gdbWidget = widgets.NewGDBWidget()
	a.gdbWidget.SetClipboard(a.ClipboardIO())
	a.gdbWidget.SetPromptStyleToken(promptTok)
	// make/gcc (and Delve listings) emit ANSI SGR on the debugger PTY.
	a.gdbWidget.SetANSI(true)
	if a.cfg.IsDLV() {
		// Also paint program stdout in the Delve console (IO pane is primary).
		a.Debug().SetGdbTargetPrint(true)
	}
	a.gdbWidget.SetOnSubmit(a.onGdbConsoleSubmit)
	a.gdbWidget.SetOnInterrupt(a.onGdbConsoleInterrupt)
	a.gdbWidget.SetOnSuspend(a.onGdbConsoleSuspend)
	a.gdbWidget.SetOnEOF(a.onGdbConsoleEOF)
	// Replay banner / -x output captured before the UI subscribed, then attach live PTY.
	if boot != "" {
		a.handleDebuggerOutputMsg(events.GdbOutputMsg{Data: boot})
	}
	a.startGdbConsoleBridge()
	a.registerBuiltin("gdb", a.gdbWidget)

	a.outputWidget = widgets.NewOutputWidget()
	a.outputWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("io", a.outputWidget)
	a.registerBuiltin("output", a.outputWidget) // alias for :b io
	a.maybeClearOutput()
	if extTTY != "" {
		a.markIOExternal(extTTY)
	} else if inferior != nil {
		a.wireInferiorIO(inferior)
	}

	savedBPs, err := persist.LoadBreakpoints(".")
	if err != nil && a.ctx.Log != nil {
		a.ctx.Log.Named("breakpoints").Error("load saved breakpoints: " + err.Error())
	}
	// Scripts via -x already set breakpoints; skip default break main when
	// restoring a saved session or using -x.
	if !skipBreakMain && len(savedBPs) == 0 {
		a.maybeBreakMain()
	}

	a.breakpoints = &models.BreakpointList{}
	a.threads = &models.ThreadList{}
	a.callstack = &models.CallStack{}
	a.assembly = &models.AssemblyList{}
	a.assemblyWidget = widgets.NewAssemblyWidget()
	a.assemblyWidget.SetClipboard(a.ClipboardIO())
	a.assemblyWidget.SetAppState(a.Debug())
	a.assemblyWidget.OnBrowse = func(addr string, rows int) {
		go a.runAssemblyRefresh(addr, rows, false)
	}
	a.assemblyWidget.OnBreakToggle = a.onAsmBreakToggle
	a.assemblyWidget.OnToggleEnable = a.toggleAsmBreakEnable
	a.registerBuiltin("asm", a.assemblyWidget)
	a.registerBuiltin("assembly", a.assemblyWidget)

	a.bpWidget = widgets.NewBreakpointWidget()
	a.bpWidget.SetClipboard(a.ClipboardIO())
	a.bpWidget.SetAppState(a.Debug())
	a.bpWidget.OnToggle = a.onBreakpointToggle
	a.bpWidget.OnDelete = a.onBreakpointDelete
	a.bpWidget.OnActivate = a.onBreakpointActivate
	a.registerBuiltin("breakpoint", a.bpWidget)

	a.threadWidget = widgets.NewThreadWidget()
	a.threadWidget.SetClipboard(a.ClipboardIO())
	a.threadWidget.SetAppState(a.Debug())
	a.threadWidget.OnActivate = a.onThreadActivate
	a.registerBuiltin("threads", a.threadWidget)

	a.callstackWidget = widgets.NewCallStackWidget()
	a.callstackWidget.SetClipboard(a.ClipboardIO())
	a.callstackWidget.SetAppState(a.Debug())
	a.callstackWidget.OnActivate = a.onCallStackActivate
	a.registerBuiltin("callstack", a.callstackWidget)

	a.fileListWidget = widgets.NewFileListWidget()
	a.fileListWidget.SetClipboard(a.ClipboardIO())
	a.fileListWidget.SetAppState(a.Debug())
	a.fileListWidget.OnOpen = a.openSourcePath

	a.luaCmds = make(map[string]*luahost.Runtime)
	a.loadUserLuaScripts()

	a.registerLayouts()

	a.gdbMcp = mcp.NewGdbMcpService(a.GDB(), a.State())
	a.gdbMcp.OnBreakpointsChanged = a.onBreakpointsChanged
	a.gdbMcp.SetDomain(appDebugDomain{app: a})
	if a.cfg.IsDLV() {
		a.gdbMcp.SetPromptToken(dlv.PromptToken)
	}
	if a.ctx.Bus != nil {
		platform.Subscribe(a.ctx.Bus, a.onBreakpointsChangedMsg)
	}
	a.restoreSavedBreakpoints(savedBPs)
	return nil
}

func (a *DebuggerApp) registerBuiltin(name string, w termui.Widget) {
	if a.builtins == nil {
		a.builtins = make(map[string]termui.Widget)
	}
	a.builtins[name] = w
}

const widgetJumpMax = 32

// swapFocusedWidget replaces the focused pane's widget and pushes the previous
// one onto the jump list (for Ctrl-O). Refuses when the focused leaf is the
// fixed GDB layout slot and w is not gdbWidget.
func (a *DebuggerApp) swapFocusedWidget(w termui.Widget) bool {
	if a.tab == nil || w == nil {
		return false
	}
	if a.isGdbLeaf(a.focusedLeaf()) && w != a.gdbWidget {
		return false
	}
	prev := a.focusedWidget()
	if prev == w {
		return false
	}
	if !a.tab.ReplaceFocusedWidget(w) {
		return false
	}
	if prev != nil {
		a.pushWidgetJump(prev)
	}
	a.rememberCodeLeafFromFocus()
	return true
}

func (a *DebuggerApp) pushWidgetJump(w termui.Widget) {
	if w == nil {
		return
	}
	// Avoid consecutive duplicates.
	if n := len(a.widgetJump); n > 0 && a.widgetJump[n-1] == w {
		return
	}
	a.widgetJump = append(a.widgetJump, w)
	if len(a.widgetJump) > widgetJumpMax {
		a.widgetJump = a.widgetJump[len(a.widgetJump)-widgetJumpMax:]
	}
}

// JumpBack restores the previous widget in the focused pane (Vim Ctrl-O).
// Leaves the jump stack untouched when the focused leaf is the GDB slot and
// the restore target is not gdbWidget.
func (a *DebuggerApp) JumpBack(args ...any) {
	if a.tab == nil || len(a.widgetJump) == 0 {
		return
	}
	prev := a.widgetJump[len(a.widgetJump)-1]
	if a.isGdbLeaf(a.focusedLeaf()) && prev != a.gdbWidget {
		return
	}
	a.widgetJump = a.widgetJump[:len(a.widgetJump)-1]
	if a.tab.ReplaceFocusedWidget(prev) {
		a.RequestFrame()
	}
}
