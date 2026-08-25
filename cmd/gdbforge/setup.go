package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

func (a *DebuggerApp) InitB() error {
	a.ctx = platform.NewAppContext()
	a.debug = debugstate.New(a.State())

	if path := a.cfg.LogFile; path != "" {
		if err := a.enableFileLog(path); err != nil {
			return err
		}
	}
	a.miLog = a.ctx.Log.Named("gdb-mi")

	if err := a.initBuiltins(); err != nil {
		return err
	}

	logo := a.logoWidget
	if logo == nil {
		logo = widgets.NewLogoWidget()
		a.logoWidget = logo
	}
	initLayoutShell(a, a.newStartupTab(logo))

	tab := a.Tab()
	tab.SetStatusClipboard(a.ClipboardIO())
	tab.FocusWidget(a.gdbWidget)
	tab.SetLeafMark(leafMarkCode, tab.FindLeaf(isCodeSlot))
	tab.SetLeafMark(leafMarkGDB, tab.FindLeaf(func(w termui.Widget) bool { return w == a.gdbWidget }))
	a.EnterInsertMode()
	tab.SetOnResize(a.RequestFrame)
	a.State().SetEqualAlways(true)
	tab.SetEqualAlways(true)
	a.AddWidget(a.Widget())

	bar := termui.NewCompletionBarWidget(a.ctx)
	a.comp.attach(&termui.CompletionMenu{}, bar)
	a.AddWidget(bar)

	a.cmdWidget = termui.NewCmdWidget(a.commandReg)
	a.cmdWidget.Ctx = a.ctx
	a.cmdWidget.SetPostInterrupt(a.PostInterrupt)
	a.cmdWidget.SetClipboard(a.ClipboardIO())
	a.cmdWidget.SetOnExecute(func() {
		_ = a.cmdWidget.ExecuteParsed()
	})
	a.cmdWidget.SetOnChange(func(text string) {
		a.search.onCmdChange(text)
	})
	a.cmdWidget.SetOnSearchSubmit(func(pattern string) {
		a.search.onCmdSubmit(pattern)
	})
	a.restoreCmdlineHistory()
	a.AddWidget(a.cmdWidget)

	a.registerUIComponents()

	a.InitKeyBindings()
	a.ExapData()

	a.RegisterModeHandler(platform.ModeNormal, a.withGlobalKeys(a.handleNormalKey))
	a.RegisterModeHandler(platform.ModeInsert, a.withGlobalKeys(a.handleInsertKey))
	a.RegisterModeHandler(platform.ModeCommand, a.withGlobalKeys(a.handleCommandKey))
	a.RegisterModeHandler(platform.ModeSearch, a.withGlobalKeys(a.handleSearchKey))
	a.RegisterModeHandler(platform.ModeCompletion, a.withGlobalKeys(a.handleCompletionKey))
	a.RegisterModeHandler(platform.ModeLua, a.withGlobalKeys(a.lua.handleKey))
	return nil
}
