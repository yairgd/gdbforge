package main

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/collections"
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
	trie      collections.Trie[any]
	tab       *termui.TabWidget
	cmdWidget *termui.CmdWidget
	ctx       platform.AppContext
}

func (app *DebuggerApp) BindKeySeq(fn collections.Callback, seqs ...string) {
	for _, seq := range seqs {
		app.trie.Bind(seq, fn)
	}
}

func NewDebuggerApp() *DebuggerApp {
	dbg := &DebuggerApp{}
	dbg.TermApp = termui.NewTermApp()
	dbg.TermApp.Api = dbg
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

	a.BindKeySeq(a.OnFocusLeft, "<C-w>l", "<C-w><Left>")
	a.BindKeySeq(a.OnFocusRight, "<C-w>h", "<C-w><Right>")
	a.BindKeySeq(a.OnFocusUp, "<C-w>k", "<C-w><Up>")
	a.BindKeySeq(a.OnFocusDown, "<C-w>j", "<C-w><Down>")

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
		a.trie.SearchPartial(key)
	} else {
		a.trie.ResetPartial()
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

func (app *DebuggerApp) HandleCoreEvents(ev termui.Event) {
	msg, ok := ev.(termui.CommandEvent)
	if !ok {
		return
	}

	switch msg.CommandID() {
	case termui.CmdUnknown:
		// TODO: show unknown command feedback in the UI

	case termui.CmdExitMode:
		if app.Mode() == platform.ModeCommand {
			app.SetMode(platform.ModeNormal)
			app.cmdWidget.Deativate()
		}
		// TODO: show unknown command feedback in the UI
	case cmdQuit:
		if app.tab.DeleteFocus() {
			// close last window - exit app
			app.Exit()
		}

		app.RequestRedraw()

	case cmdVerticalSplit:

		app.tab.VerticalSplit(widgets.NewCodeWidget())
		app.RequestRedraw()

	case cmdHorizontalSplit:
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

}

func main() {

	app := NewDebuggerApp()
	app.Run()

}
