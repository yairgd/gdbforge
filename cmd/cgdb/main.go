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
	cmdQuit
	cmdVerticalSplit
	cmdHorizontalSplit
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

func (app *DebuggerApp) SplitHorizontal(args ...any) {}
func (app *DebuggerApp) SplitVertical(args ...any)   {}
func (app *DebuggerApp) BreakFileLine(args ...any)   {}
func (app *DebuggerApp) BreakFunction(args ...any)   {}

// ExapData builds the example command hierarchy on commandReg.Root:
//
//	/ → window → left, right, up, down
//	/ → break  → file, delete
//	/ → info   → registers, threads
func (a *DebuggerApp) ExapData() {
	root := a.commandReg.Root

	window := root.InsertName("window")
	window.InsertName("left").Action = a.OnFocusLeft
	window.InsertName("right").Action = a.OnFocusRight
	window.InsertName("up").Action = a.OnFocusUp
	window.InsertName("down").Action = a.OnFocusDown

	breakCmd := root.InsertName("break")
	breakCmd.InsertName("file").Action = a.BreakFile
	breakCmd.InsertName("delete").Action = a.DeleteBreakpoint

	info := root.InsertName("info")
	info.InsertName("registers").Action = a.ShowRegisters
	info.InsertName("threads").Action = a.ShowThreads
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

	completer := termui.NewSimpleCompleter([]termui.Command{
		{ID: cmdBreak, Name: "break"},
		{ID: cmdContinue, Name: "continue"},
		{ID: cmdNext, Name: "next"},
		{ID: cmdStep, Name: "step"},
		{ID: cmdPrint, Name: "print"},
		{ID: cmdBacktrace, Name: "bt"},
		{ID: cmdInfo, Name: "info"},
		{ID: cmdRun, Name: "run"},
		{ID: cmdQuit, Name: "quit"},
		{ID: cmdVerticalSplit, Name: "vs"},
		{ID: cmdHorizontalSplit, Name: "split"},
	})
	a.cmdWidget = termui.NewCmdWidget(completer)
	a.cmdWidget.Events = a.Events()
	a.AddWidget(a.cmdWidget)

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

	a.ExapData()
	a.ctx = platform.NewAppContext()

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
	a.RegisterCommandHandler(cmdQuit, a.handleQuit)
	a.RegisterCommandHandler(cmdVerticalSplit, a.handleVerticalSplit)
	a.RegisterCommandHandler(cmdHorizontalSplit, a.handleHorizontalSplit)
}

func (a *DebuggerApp) handleInsertKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyEscape {
		a.SetMode(platform.ModeNormal)
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
		a.SetMode(platform.ModeInsert)
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
		a.SetMode(platform.ModeNormal)
		a.cmdWidget.Deativate()
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
			a.SetMode(platform.ModeInsert)
		}
	}

	if ev.Buttons()&(tcell.WheelUp|tcell.WheelDown) != 0 {
		x, y := ev.Position()
		if a.tab.FocusAt(x, y) {
			a.SetMode(platform.ModeInsert)
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

func (app *DebuggerApp) handleQuit(_ termui.CommandEvent) bool {
	if app.tab.DeleteFocus() {
		app.Exit()
	}
	app.RequestRedraw()
	return true
}

func (app *DebuggerApp) handleVerticalSplit(_ termui.CommandEvent) bool {
	app.tab.VerticalSplit(widgets.NewCodeWidget())
	app.RequestRedraw()
	return true
}

func (app *DebuggerApp) handleHorizontalSplit(_ termui.CommandEvent) bool {
	w := app.Widgets()[0].Widget()
	tab, ok := w.(*termui.TabWidget)
	if !ok {
		return true
	}

	l := widgets.NewLoggerWidget(app.ctx)
	l.Events = app.Events()
	l.SetCopyToClipboard(app.CopyToClipboard)
	tab.HorizontalSplit(l)
	app.RequestRedraw()
	return true
}

// exampleCommandDSL demonstrates a future declarative syntax for building the
// command tree. It is not wired into command execution, key bindings, or the
// colon-command parser — the returned registry is discarded after construction.
//
// Compared to repeated InsertName() calls, this DSL reads as a nested outline:
// groups and commands mirror the logical hierarchy (window → split → horizontal)
// instead of spelling out each parent/child link imperatively.
//
// Each CommandNode still owns a Children trie internally; the DSL never exposes
// that detail. The trie is an implementation mechanism for efficient child lookup
// and future auto-completion, while Group/Cmd describe the command tree itself.
func exampleCommandDSL(app *DebuggerApp) *commands.CommandRegistry {
	registry := commands.NewCommandRegistry()

	registry.Root.
		Group("window",
			commands.Cmd("left", app.OnFocusLeft),
			commands.Cmd("right", app.OnFocusRight),
			commands.Cmd("up", app.OnFocusUp),
			commands.Cmd("down", app.OnFocusDown),

			commands.Group("split",
				commands.Cmd("horizontal", app.SplitHorizontal),
				commands.Cmd("vertical", app.SplitVertical),
			),
		).
		Group("break",
			commands.Group("file",
				commands.Cmd("line", app.BreakFileLine),
				commands.Cmd("function", app.BreakFunction),
			),

			commands.Cmd("delete", app.DeleteBreakpoint),
		).
		Group("info",
			commands.Cmd("registers", app.ShowRegisters),
			commands.Cmd("threads", app.ShowThreads),
		)

	return registry
}

func main() {

	app := NewDebuggerApp()
	_ = exampleCommandDSL(app) // DSL demo only; not connected to the running app
	app.Run()

}
