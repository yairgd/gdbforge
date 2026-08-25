package main

import (
	"github.com/yairgd/gdbforge/internal/demo"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

func (a *DemoApp) Init() error {
	a.ctx = platform.NewAppContext()

	a.mainPane = termui.NewConsolePane("main")
	a.mainPane.SetClipboard(a.ClipboardIO())
	a.mainPane.SetInputEnabled(true)
	a.mainPane.Buffer().AppendLine("demo — host showcase. Press :help or type in this pane (i).")
	a.builtins["main"] = a.mainPane

	a.sidePane = termui.NewConsolePane("side")
	a.sidePane.SetInputEnabled(false)
	a.sidePane.Buffer().AppendLine("side pane — :b side")
	a.sidePane.Buffer().AppendLine("status / notes go here")
	a.builtins["side"] = a.sidePane

	a.logPane = termui.NewLoggerWidget(a.ctx)
	a.logPane.PaneName = "log"
	a.builtins["log"] = a.logPane
	a.ctx.Log.Named("demo").Info("demo started")

	a.tab = demo.BuildDefault("demo", demo.Panes{
		Main: a.mainPane,
		Side: a.sidePane,
		Log:  a.logPane,
	})
	a.tab.SetStatusClipboard(a.ClipboardIO())
	a.tab.FocusWidget(a.mainPane)
	a.EnterInsertMode()
	a.tab.SetOnResize(a.RequestFrame)
	a.State().SetEqualAlways(true)
	a.tab.SetEqualAlways(true)
	a.AddWidget(a.tab)

	a.AddWidget(termui.NewCompletionBarWidget(a.ctx))

	a.cmdWidget = termui.NewCmdWidget(a.commandReg)
	a.cmdWidget.Ctx = a.ctx
	a.cmdWidget.SetPostInterrupt(a.PostInterrupt)
	a.cmdWidget.SetClipboard(a.ClipboardIO())
	a.cmdWidget.SetOnExecute(func() {
		_ = a.cmdWidget.ExecuteParsed()
	})
	a.AddWidget(a.cmdWidget)

	if a.ctx.Bus != nil {
		platform.Subscribe(a.ctx.Bus, func(msg termui.SubmitMsg) {
			switch msg.CmdID {
			case termui.CmdExitMode:
				a.leaveCommandMode()
			}
		})
	}

	a.InitKeyBindings()
	a.ExapData()

	a.RegisterModeHandler(platform.ModeNormal, a.handleNormalKey)
	a.RegisterModeHandler(platform.ModeInsert, a.handleInsertKey)
	a.RegisterModeHandler(platform.ModeCommand, a.handleCommandKey)
	return nil
}
