package main

import (
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/mcp"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

// initBuiltins creates singleton built-in views once at startup.
// Adding a new page: construct it here, registerBuiltin(name, w).
// Show with :b name (OnBuffer). Source files use :edit filename (per-file CodeWidget).
func (a *DebuggerApp) initBuiltins() error {
	a.builtins = make(map[string]termui.Widget)
	a.fileBuffers = make(map[string]*widgets.CodeWidget)

	a.aboutWidget = widgets.NewAboutWidget()
	a.registerBuiltin("about", a.aboutWidget)

	logWidget := termui.NewLoggerWidget(a.ctx)
	logWidget.Events = a.Events()
	logWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("logger", logWidget)

	gdbWidget, err := widgets.NewGDBWidget(a.cfg.GDBPath, a.cfg.Prog, a.cfg.ProgArgs...)
	if err != nil {
		return err
	}
	a.gdbWidget = gdbWidget
	a.gdbWidget.SetClipboard(a.ClipboardIO())
	a.gdbWidget.SetAppState(a.State())
	a.gdbWidget.SetOnStopped(a.onGdbStopped)
	a.gdbWidget.SetOnBreakpointsChanged(a.onBreakpointsChanged)
	a.gdbWidget.Start(a.Screen())
	a.registerBuiltin("gdb", a.gdbWidget)

	a.outputWidget = widgets.NewOutputWidget()
	a.outputWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("output", a.outputWidget)
	a.maybeClearOutput()

	a.bpWidget = widgets.NewBreakpointWidget()
	a.bpWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("breakpoint", a.bpWidget)

	a.threadWidget = widgets.NewThreadWidget()
	a.threadWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("threads", a.threadWidget)

	a.callstackWidget = widgets.NewCallStackWidget()
	a.callstackWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("callstack", a.callstackWidget)

	a.fileListWidget = widgets.NewFileListWidget()
	a.fileListWidget.SetClipboard(a.ClipboardIO())
	a.fileListWidget.SetAppState(a.State())
	a.fileListWidget.OnOpen = a.openSourcePath

	a.State().RegisterLayout(platform.LayoutDefault)
	a.State().SetCurrentLayout(platform.LayoutDefault)

	a.gdbMcp = mcp.NewGdbMcpService(a.gdbWidget.Session(), a.State())
	a.gdbMcp.OnBreakpointsChanged = a.onBreakpointsChanged
	a.bpWidget.SetPTY(a.gdbWidget.Session(), a.State())
	a.bpWidget.OnChange = a.onBreakpointListChanged
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
	prev := a.tab.FocusedWidget()
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
