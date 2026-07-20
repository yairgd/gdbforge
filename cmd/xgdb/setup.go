package main

import (
	"github.com/yairgd/cgdb-go/internal/xgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

func (a *DebuggerApp) InitB() error {
	a.ctx = platform.NewAppContext()

	fileSink, err := platform.NewFileSink("xgdb.log")
	if err != nil {
		panic(err)
	}
	a.ctx.Log.AddSink(fileSink)
	a.miLog = a.ctx.Log.Named("gdb-mi")

	if err := a.initBuiltins(); err != nil {
		return err
	}

	logo := a.logoWidget
	if logo == nil {
		logo = widgets.NewLogoWidget()
		a.logoWidget = logo
	}

	a.tab = a.newStartupTab(logo)
	// FocusDown needs layout geometry; set GDB focus by widget before first paint.
	a.tab.FocusWidget(a.gdbWidget)
	a.tab.SetLeafMark(leafMarkCode, a.tab.FindLeaf(isCodeSlot))
	a.tab.SetLeafMark(leafMarkGDB, a.tab.FindLeaf(func(w termui.Widget) bool { return w == a.gdbWidget }))
	a.EnterInsertMode()
	a.tab.SetOnResize(a.RequestFrame)
	a.State().SetEqualAlways(true)
	a.tab.SetEqualAlways(true)
	a.AddWidget(a.tab)

	a.completionBar = termui.NewCompletionBarWidget(a.ctx)
	a.completionBar.Events = a.Events()
	a.AddWidget(a.completionBar)

	a.cmdWidget = termui.NewCmdWidget(a.commandReg)
	a.cmdWidget.Ctx = a.ctx
	a.cmdWidget.Events = a.Events()
	a.cmdWidget.SetClipboard(a.ClipboardIO())
	a.AddWidget(a.cmdWidget)

	a.InitKeyBindings()
	a.ExapData()

	a.RegisterModeHandler(platform.ModeNormal, a.handleNormalKey)
	a.RegisterModeHandler(platform.ModeInsert, a.handleInsertKey)
	a.RegisterModeHandler(platform.ModeCommand, a.handleCommandKey)
	a.RegisterModeHandler(platform.ModeCompletion, a.handleCompletionKey)

	a.RegisterCommandHandler(termui.CmdUnknown, a.handleUnknownCommand)
	a.RegisterCommandHandler(termui.CmdExitMode, a.handleExitMode)
	return nil
}
