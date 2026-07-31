package main

import (
	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
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
	a.bufs.initMaps()

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
		a.backend = backend.NewDLV(client)
	} else {
		client, err := gdb.NewGDBClientOpts(a.cfg.GDBPath, a.cfg.GDBArgs, gdb.ClientOptions{InferiorTTY: extTTY})
		if err != nil {
			return err
		}
		a.backend = backend.NewGDB(client)
		skipBreakMain = gdb.HasInitScript(a.cfg.GDBArgs)
	}
	boot = a.backend.TakeStartupOutput()
	inferior = a.backend.InferiorTTY()
	promptTok = a.backend.PromptToken()
	_ = a.backend.ConfigureInferiorTTY()

	a.gdbWidget = widgets.NewGDBWidget()
	a.gdbWidget.SetClipboard(a.ClipboardIO())
	a.gdbWidget.SetPromptStyleToken(promptTok)
	// make/gcc (and Delve listings) emit ANSI SGR on the debugger PTY.
	a.gdbWidget.SetANSI(true)
	if a.backend.PaintTargetInConsole() {
		// Also paint program stdout in the Delve console (IO pane is primary).
		a.Debug().SetGdbTargetPrint(true)
	}
	wireConsole(a.gdbWidget, consoleHandlers{
		Submit:    a.console.onGdbConsoleSubmit,
		Interrupt: a.console.onGdbConsoleInterrupt,
		Suspend:   a.console.onGdbConsoleSuspend,
		EOF:       a.console.onGdbConsoleEOF,
	})
	// Replay banner / -x output captured before the UI subscribed, then attach live PTY.
	if boot != "" {
		a.console.handleDebuggerOutputMsg(events.GdbOutputMsg{Data: boot})
	}
	a.console.startGdbConsoleBridge()
	a.registerBuiltin("gdb", a.gdbWidget)

	a.outputWidget = widgets.NewOutputWidget()
	a.outputWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("io", a.outputWidget)
	a.registerBuiltin("output", a.outputWidget) // alias for :b io
	a.maybeClearOutput()
	if extTTY != "" {
		a.inferiorIO.markExternal(extTTY)
	} else if inferior != nil {
		a.inferiorIO.wire(inferior)
	}

	savedBPs, err := persist.LoadBreakpoints(".")
	if err != nil && a.ctx.Log != nil {
		a.ctx.Log.Named("breakpoints").Error("load saved breakpoints: " + err.Error())
	}
	// Scripts via -x already set breakpoints; skip default break main when
	// restoring a saved session or using -x.
	if !skipBreakMain && len(savedBPs) == 0 {
		a.breaks.maybeBreakMain()
	}

	a.breaks.list = &models.BreakpointList{}
	a.debugInfo.threads = &models.ThreadList{}
	a.debugInfo.stack = &models.CallStack{}
	a.asm.list = &models.AssemblyList{}
	a.asm.widget = widgets.NewAssemblyWidget(a)
	a.asm.widget.SetClipboard(a.ClipboardIO())
	a.asm.widget.SetAppState(a.Debug())
	a.registerBuiltin("asm", a.asm.widget)
	a.registerBuiltin("assembly", a.asm.widget)

	a.bpWidget = widgets.NewBreakpointWidget(a)
	a.bpWidget.SetClipboard(a.ClipboardIO())
	a.bpWidget.SetAppState(a.Debug())
	a.registerBuiltin("breakpoint", a.bpWidget)

	a.debugInfo.threadW = widgets.NewThreadWidget(a)
	a.debugInfo.threadW.SetClipboard(a.ClipboardIO())
	a.debugInfo.threadW.SetAppState(a.Debug())
	a.registerBuiltin("threads", a.debugInfo.threadW)

	a.debugInfo.stackW = widgets.NewCallStackWidget(a)
	a.debugInfo.stackW.SetClipboard(a.ClipboardIO())
	a.debugInfo.stackW.SetAppState(a.Debug())
	a.registerBuiltin("callstack", a.debugInfo.stackW)

	a.fileListWidget = widgets.NewFileListWidget(a)
	a.fileListWidget.SetClipboard(a.ClipboardIO())
	a.fileListWidget.SetAppState(a.Debug())
	a.lua.cmds = make(map[string]*luahost.Runtime)
	a.lua.loadScripts()

	a.registerLayouts()

	a.gdbMcp = mcp.NewGdbMcpService(a.GDB(), a.State())
	a.gdbMcp.OnBreakpointsChanged = a.breaks.onChanged
	a.gdbMcp.SetDomain(appDebugDomain{app: a})
	if a.cfg.IsDLV() {
		a.gdbMcp.SetPromptToken(dlv.PromptToken)
	}
	if a.ctx.Bus != nil {
		platform.Subscribe(a.ctx.Bus, a.breaks.onChangedMsg)
	}
	a.breaks.restoreSaved(savedBPs)
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
