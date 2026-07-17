package main

import (
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/execcli"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

func (app *DebuggerApp) OnFocusLeft(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send left command")
	app.tab.FocusLeft()
}

func (app *DebuggerApp) OnFocusRight(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send right command")

	app.tab.FocusRight()
}

func (app *DebuggerApp) OnFocusUp(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send up command")

	app.tab.FocusUp()
}

func (app *DebuggerApp) OnFocusDown(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send down command")

	app.tab.FocusDown()
}

func (app *DebuggerApp) BreakFile(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("break file")
}

func (app *DebuggerApp) DeleteBreakpoint(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("delete breakpoint")
}

func (app *DebuggerApp) ShowRegisters(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("show registers")
}

func (app *DebuggerApp) ShowThreads(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("show threads")
}

func (app *DebuggerApp) SplitHorizontal(args ...any) {
	w := app.Widgets()[0].Widget()
	tab, ok := w.(*termui.TabWidget)
	if !ok {
		return
	}

	l := termui.NewLoggerWidget(app.ctx)
	l.Events = app.Events()
	l.SetClipboard(app.ClipboardIO())
	tab.HorizontalSplit(l)
	app.RequestRedraw()
}

func (app *DebuggerApp) SplitVertical(args ...any) {
	app.tab.VerticalSplit(widgets.NewCodeWidget())
	app.RequestRedraw()
}

func (app *DebuggerApp) EnterInsertMode(args ...any) {
	app.tab.SetInsertActive(true)
	app.SetMode(platform.ModeInsert)
	app.RequestRedraw()
}

func (app *DebuggerApp) ClearFocus(args ...any) {
	w := app.tab.FocusedWidget()
	if c, ok := w.(termui.Clearable); ok {
		c.Clear()
	}
	app.RequestFrame()
}

func (app *DebuggerApp) Quit(args ...any) {
	if app.tab.DeleteFocus() {
		if app.gdbClient != nil {
			app.gdbClient.Close()
		}
		if app.execClient != nil {
			app.execClient.Close()
		}
		app.Exit()
	}
	app.RequestRedraw()
}

// OnRun starts (or restarts) an ExecClient for the given argv and shows ExecWidget
// in the focused pane. Example: :!bash  or  :!ssh root@192.168.20.50
func (app *DebuggerApp) OnRun(args ...any) {
	argv := make([]string, 0, len(args))
	for _, a := range args {
		s, ok := a.(string)
		if !ok || s == "" {
			continue
		}
		argv = append(argv, s)
	}
	if len(argv) == 0 {
		return
	}

	if app.execClient != nil {
		app.execClient.Close()
		app.execClient = nil
	}

	client, outputChan, err := execcli.NewExecClient(argv)
	if err != nil {
		if app.ctx.Log != nil {
			app.ctx.Log.Named("exec").Error(err.Error())
		}
		return
	}
	app.execClient = client

	w := widgets.NewExecWidget(client)
	w.SetClipboard(app.ClipboardIO())
	w.SetSizeFunc(client.SetSize)
	w.SetOnClose(func() {
		client.Close()
	})
	w.StartExecUIBridge(app.Screen(), outputChan)
	app.execWidget = w
	app.registerBuiltin("exec", w)

	if app.tab != nil && app.swapFocusedWidget(w) {
		app.EnterInsertMode()
		app.RequestFrame()
	}
}
