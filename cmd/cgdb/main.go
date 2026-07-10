package main

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/commands"
	"github.com/yairgd/cgdb-go/internal/platform"

	"github.com/yairgd/cgdb-go/internal/termui"
)

const (
	cmdBreak termui.CommandID = iota + 2
	cmdContinue
	cmdNext
	cmdStep
	cmdPrint
	cmdBacktrace
	cmdInfo
	cmdRun
)

type DebuggerApp struct {
	*termui.TermApp
	commandReg  *commands.CommandRegistry
	keyBindings *commands.KeyBindingRegistry

	tab       *termui.TabWidget
	cmdWidget *termui.CmdWidget
	ctx       platform.AppContext
}

//func (app *DebuggerApp) BindKeySeq(fn collections.Callback, seqs ...string) {
//	for _, seq := range seqs {
//		app.trie.Bind(seq, fn)
//	}
//}

func NewDebuggerApp() *DebuggerApp {
	dbg := &DebuggerApp{}
	dbg.TermApp = termui.NewTermApp()
	dbg.TermApp.Api = dbg
	dbg.commandReg = commands.NewCommandRegistry()
	dbg.InitB()
	dbg.HandleResize()
	return dbg
}

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

// ExapData builds the example command hierarchy on commandReg.Root:
//
//	/ → window → left, right, up, down
//	/ → break  → file, delete
//	/ → info   → registers, threads
//	/ → vs, split, i, quit
func (a *DebuggerApp) ExapData() {
	a.commandReg.Root.
		Group("window",
			commands.Cmd("left", a.OnFocusLeft),
			commands.Cmd("right", a.OnFocusRight),
			commands.Cmd("up", a.OnFocusUp),
			commands.Cmd("down", a.OnFocusDown),
		).
		Group("break",
			commands.Cmd("file", a.BreakFile),
			commands.Cmd("delete", a.DeleteBreakpoint),
		).
		Group("info",
			commands.Cmd("registers", a.ShowRegisters),
			commands.Cmd("threads", a.ShowThreads),
		).
		Leaf("vs", a.SplitVertical).
		Leaf("split", a.SplitHorizontal).
		Leaf("quit", a.Quit)

}

func (a *DebuggerApp) InitKeyBindings() {
	a.keyBindings = commands.NewKeyBindingRegistry()

	a.keyBindings.Bind(
		commands.NewCommand("move-left", func(args ...any) {
			a.OnFocusLeft()
		}),
		"<C-w>l", "<C-w><Left>",
	)

	a.keyBindings.Bind(
		commands.NewCommand("move-right", func(args ...any) {
			a.OnFocusRight()
		}),
		"<C-w>h", "<C-w><Right>",
	)

	a.keyBindings.Bind(
		commands.NewCommand("move-up", func(args ...any) {
			a.OnFocusUp()
		}),
		"<C-w>k", "<C-w><Up>",
	)

	a.keyBindings.Bind(
		commands.NewCommand("move-down", func(args ...any) {
			a.OnFocusDown()
		}),
		"<C-w>j", "<C-w><Down>",
	)
}

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

	a.cmdWidget = termui.NewCmdWidget(
		a.commandReg,
		termui.NewLogCompletionPresenter(a.ctx.Log.Named("CmdLine")),
	)
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

func (a *DebuggerApp) handleInsertKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyEscape {
		a.tab.SetInsertActive(false)
		a.SetMode(platform.ModeNormal)
		a.RequestRedraw()
		return true
	}
	a.tab.HandleEvent(ev)
	return true
}

func (a *DebuggerApp) handleNormalKey(ev *tcell.EventKey) bool {
	if isCopyKey(ev) {
		a.tab.HandleEvent(ev)
		return true
	}
	if ev.Key() == tcell.KeyRune && ev.Rune() == ':' {
		a.SetMode(platform.ModeCommand)
		a.cmdWidget.Activate()
		return true
	}
	if ev.Key() == tcell.KeyRune && ev.Rune() == 'i' {
		a.EnterInsertMode()
		return true
	}
	if key, ok := platform.KeyFromEvent(ev); ok {
		cmd, ok := a.keyBindings.SearchPartial(key)
		if ok {
			cmd.Action()
		}
	} else {
		a.keyBindings.ResetPartial()
	}
	return true
}

func (a *DebuggerApp) handleCommandKey(ev *tcell.EventKey) bool {
	a.cmdWidget.HandleEvent(ev)
	if ev.Key() == tcell.KeyEnter {
		a.cmdWidget.Deativate()
		if a.Mode() == platform.ModeCommand {
			a.SetMode(platform.ModeNormal)
		}
	}
	return true
}

func isCopyKey(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyCtrlC ||
		(ev.Key() == tcell.KeyRune && ev.Rune() == 'c' && ev.Modifiers()&tcell.ModCtrl != 0)
}

func (a *DebuggerApp) HandleMouse(ev *tcell.EventMouse) {
	if a.Mode() == platform.ModeCommand {
		return
	}

	if ev.Buttons()&tcell.ButtonPrimary != 0 {
		x, y := ev.Position()
		if !a.tab.IsSeparatorAt(x, y) && a.tab.FocusAt(x, y) {
			a.EnterInsertMode()
		}
	}

	if ev.Buttons()&(tcell.WheelUp|tcell.WheelDown) != 0 {
		x, y := ev.Position()
		if a.tab.FocusAt(x, y) {
			a.EnterInsertMode()
		}
	}

	a.tab.HandleEvent(ev)
}

func (a *DebuggerApp) HandleResize() {

	c := a.UpdateCanvas()

	w := a.Widgets()
	if len(w) < 2 {
		return
	}
	w[0].SetRect(c.ChildRect(0, 0, c.W(), c.H()))
	w[1].SetRect(c.ChildRect(0, c.H()-1, c.W(), 1))
}

func (app *DebuggerApp) handleUnknownCommand(_ termui.CommandEvent) bool {
	// TODO: show unknown command feedback in the UI
	return true
}

func (app *DebuggerApp) handleExitMode(_ termui.CommandEvent) bool {
	if app.Mode() == platform.ModeCommand {
		app.SetMode(platform.ModeNormal)
		app.cmdWidget.Deativate()
	}
	return true
}

func main() {
	app := NewDebuggerApp()
	app.Run()
}
