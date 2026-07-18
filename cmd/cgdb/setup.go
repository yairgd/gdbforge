package main

import (
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

func (a *DebuggerApp) InitB() error {
	a.ctx = platform.NewAppContext()

	fileSink, err := platform.NewFileSink("cgdb.log")
	if err != nil {
		panic(err)
	}
	a.ctx.Log.AddSink(fileSink)
	a.miLog = a.ctx.Log.Named("gdb-mi")

	logWidget := termui.NewLoggerWidget(a.ctx)
	logWidget.Events = a.Events()
	logWidget.SetClipboard(a.ClipboardIO())

	if err := a.initBuiltins(); err != nil {
		return err
	}

	a.tab = termui.NewTabTwoHozSplitWins(
		"basic debugger",
		logWidget,
		a.gdbWidget,
	)
	a.tab.FocusDown()
	a.tab.SetOnResize(a.RequestFrame)
	a.AddWidget(a.tab)

	a.cmdWidget = termui.NewCmdWidget(a.commandReg)
	a.cmdWidget.Ctx = a.ctx
	a.cmdWidget.Events = a.Events()
	a.AddWidget(a.cmdWidget)

	a.InitKeyBindings()
	a.ExapData()

	a.RegisterModeHandler(platform.ModeNormal, a.handleNormalKey)
	a.RegisterModeHandler(platform.ModeInsert, a.handleInsertKey)
	a.RegisterModeHandler(platform.ModeCommand, a.handleCommandKey)

	a.RegisterCommandHandler(termui.CmdUnknown, a.handleUnknownCommand)
	a.RegisterCommandHandler(termui.CmdExitMode, a.handleExitMode)
	return nil
}
