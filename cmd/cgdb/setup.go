package main

import (
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

func (a *DebuggerApp) InitB() {
	codeWidgetLeft := widgets.NewCodeWidget()
	codeWidgetRight := widgets.NewCodeWidget()

	a.tab = termui.NewTabTwoHozSplitWins(
		"basic debugger",
		codeWidgetLeft,
		codeWidgetRight,
	)
	a.tab.SetOnResize(a.RequestFrame)
	a.AddWidget(a.tab)

	a.ctx = platform.NewAppContext()

	a.cmdWidget = termui.NewCmdWidget(a.commandReg)
	a.cmdWidget.Ctx = a.ctx
	a.cmdWidget.Events = a.Events()
	a.AddWidget(a.cmdWidget)

	a.InitKeyBindings()
	a.ExapData()

	// exaplr how to use filesynk
	fileSink, err := platform.NewFileSink("cgdb.log")
	if err != nil {
		panic(err)
	}
	defer fileSink.Close()
	a.ctx.Log.AddSink(fileSink)

	l := widgets.NewLoggerWidget(a.ctx)
	a.tab.HorizontalSplit(l)

	a.RegisterModeHandler(platform.ModeNormal, a.handleNormalKey)
	a.RegisterModeHandler(platform.ModeInsert, a.handleInsertKey)
	a.RegisterModeHandler(platform.ModeCommand, a.handleCommandKey)

	a.RegisterCommandHandler(termui.CmdUnknown, a.handleUnknownCommand)
	a.RegisterCommandHandler(termui.CmdExitMode, a.handleExitMode)
}
