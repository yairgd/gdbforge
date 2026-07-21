package main

import (
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// initBuiltins creates singleton built-in views once at startup.
// Adding a new page: construct it here, registerBuiltin(name, w).
// Show with :b name (OnBuffer). Source files use :edit filename (per-file CodeWidget).
func (a *DebuggerApp) initBuiltins() error {
	a.builtins = make(map[string]termui.Widget)
	a.fileBuffers = make(map[string]*widgets.CodeWidget)

	a.aboutWidget = widgets.NewAboutWidget()
	a.registerBuiltin("about", a.aboutWidget)

	a.helpWidget = widgets.NewHelpWidget()
	a.helpWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("help", a.helpWidget)

	a.logoWidget = widgets.NewLogoWidget()

	logWidget := termui.NewLoggerWidget(a.ctx)
	logWidget.Events = a.Events()
	logWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("logger", logWidget)

	client, err := gdb.NewGDBClient(a.cfg.GDBPath, a.cfg.Prog, a.cfg.ProgArgs...)
	if err != nil {
		return err
	}
	a.gdbClient = client
	a.gdbInputState = gdb.NewGdbInputState()

	a.gdbWidget = widgets.NewGDBWidget()
	a.gdbWidget.SetClipboard(a.ClipboardIO())
	a.gdbWidget.SetOnSubmit(a.onGdbConsoleSubmit)
	a.gdbWidget.SetOnInterrupt(a.onGdbConsoleInterrupt)
	a.gdbWidget.SetOnEOF(a.onGdbConsoleEOF)
	a.startGdbConsoleBridge()
	a.registerBuiltin("gdb", a.gdbWidget)

	a.outputWidget = widgets.NewOutputWidget()
	a.outputWidget.SetClipboard(a.ClipboardIO())
	if tty := a.gdbClient.InferiorTTY(); tty != nil {
		a.wireInferiorIO(tty)
	}
	a.registerBuiltin("io", a.outputWidget)
	a.registerBuiltin("output", a.outputWidget) // alias for :b io
	a.maybeClearOutput()
	a.maybeBreakMain()

	a.breakpoints = &models.BreakpointList{}
	a.threads = &models.ThreadList{}
	a.callstack = &models.CallStack{}
	a.bpWidget = widgets.NewBreakpointWidget()
	a.bpWidget.SetClipboard(a.ClipboardIO())
	a.bpWidget.SetAppState(a.State())
	a.bpWidget.OnToggle = a.onBreakpointToggle
	a.bpWidget.OnDelete = a.onBreakpointDelete
	a.bpWidget.OnActivate = a.onBreakpointActivate
	a.registerBuiltin("breakpoint", a.bpWidget)

	a.threadWidget = widgets.NewThreadWidget()
	a.threadWidget.SetClipboard(a.ClipboardIO())
	a.threadWidget.SetAppState(a.State())
	a.threadWidget.OnActivate = a.onThreadActivate
	a.registerBuiltin("threads", a.threadWidget)

	a.callstackWidget = widgets.NewCallStackWidget()
	a.callstackWidget.SetClipboard(a.ClipboardIO())
	a.callstackWidget.SetAppState(a.State())
	a.callstackWidget.OnActivate = a.onCallStackActivate
	a.registerBuiltin("callstack", a.callstackWidget)

	a.fileListWidget = widgets.NewFileListWidget()
	a.fileListWidget.SetClipboard(a.ClipboardIO())
	a.fileListWidget.SetAppState(a.State())
	a.fileListWidget.OnOpen = a.openSourcePath

	a.registerLayouts()

	a.gdbMcp = mcp.NewGdbMcpService(a.GDB(), a.State())
	a.gdbMcp.OnBreakpointsChanged = a.onBreakpointsChanged
	if a.ctx.Bus != nil {
		platform.Subscribe(a.ctx.Bus, a.onBreakpointsChangedMsg)
	}
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
// one onto the jump list (for Ctrl-O).
func (a *DebuggerApp) swapFocusedWidget(w termui.Widget) bool {
	if a.tab == nil || w == nil {
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
func (a *DebuggerApp) JumpBack(args ...any) {
	if a.tab == nil || len(a.widgetJump) == 0 {
		return
	}
	prev := a.widgetJump[len(a.widgetJump)-1]
	a.widgetJump = a.widgetJump[:len(a.widgetJump)-1]
	if a.tab.ReplaceFocusedWidget(prev) {
		a.RequestFrame()
	}
}
