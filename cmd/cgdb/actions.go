package main

import (
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
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

	l := widgets.NewLoggerWidget(app.ctx)
	l.Events = app.Events()
	l.SetCopyToClipboard(app.CopyToClipboard)
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

func (app *DebuggerApp) Quit(args ...any) {
	if app.tab.DeleteFocus() {
		app.Exit()
	}
	app.RequestRedraw()
}
