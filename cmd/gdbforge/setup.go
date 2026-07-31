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

	fileSink, err := platform.NewFileSink("gdbforge.log")
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

	// DebuggerApp wires the wildmenu into the layout; completionCtl owns it.
	bar := termui.NewCompletionBarWidget(a.ctx)
	bar.Events = a.Events()
	a.comp.attach(&termui.CompletionMenu{}, bar)
	a.AddWidget(bar)
	if a.ctx.Bus != nil {
		platform.Subscribe(a.ctx.Bus, a.comp.onMsg)
	}

	a.cmdWidget = termui.NewCmdWidget(a.commandReg)
	a.cmdWidget.Ctx = a.ctx
	a.cmdWidget.Events = a.Events()
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

	a.InitKeyBindings()
	a.ExapData()

	// Ctrl-Z (suspend) is global — wrap every mode so it works in any app state.
	a.RegisterModeHandler(platform.ModeNormal, a.withGlobalKeys(a.handleNormalKey))
	a.RegisterModeHandler(platform.ModeInsert, a.withGlobalKeys(a.handleInsertKey))
	a.RegisterModeHandler(platform.ModeCommand, a.withGlobalKeys(a.handleCommandKey))
	a.RegisterModeHandler(platform.ModeSearch, a.withGlobalKeys(a.handleSearchKey))
	a.RegisterModeHandler(platform.ModeCompletion, a.withGlobalKeys(a.handleCompletionKey))
	a.RegisterModeHandler(platform.ModeLua, a.withGlobalKeys(a.lua.handleKey))

	a.RegisterCommandHandler(termui.CmdUnknown, a.handleUnknownCommand)
	a.RegisterCommandHandler(termui.CmdExitMode, a.handleExitMode)
	return nil
}
