package main

import (
	"github.com/yairgd/gdbforge/internal/demo"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

func (a *DemoApp) Init() error {
	a.ctx = platform.NewAppContext()

	a.mainPane = demo.NewScrollPane("main", "demo — host showcase. Press :help.")
	a.mainPane.SetClipboard(a.ClipboardIO())
	a.builtins["main"] = a.mainPane

	a.sidePane = demo.NewScrollPane("side",
		"side pane — :b side",
		"status / notes go here",
	)
	a.sidePane.SetClipboard(a.ClipboardIO())
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
	a.tab.SetOnResize(a.RequestFrame)
	a.State().SetEqualAlways(true)
	a.tab.SetEqualAlways(true)
	a.AddWidget(a.tab)

	a.AddWidget(termui.NewCompletionBarWidget(a.ctx))

	a.cmdWidget = termui.NewCmdWidget(a.commandReg)
	a.cmdWidget.Ctx = a.ctx
	a.cmdWidget.SetPostInterrupt(a.PostInterrupt)
	a.cmdWidget.SetClipboard(a.ClipboardIO())
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
