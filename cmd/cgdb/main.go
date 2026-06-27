package main

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/cgdb"
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
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
	trie      termui.Trie
	appState  cgdb.AppState
	tab       *termui.TabWidget
	cmdWidget *termui.CmdWidget
}

func (app *DebuggerApp) BindKeySeq(fn termui.Callback, seqs ...string) {
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
	app.tab.FocusLeft()
}

func (app *DebuggerApp) OnFocusRight(args ...any) {
	app.tab.FocusRight()
}

func (app *DebuggerApp) OnFocusUp(args ...any) {
	app.tab.FocusUp()
}

func (app *DebuggerApp) OnFocusDown(args ...any) {
	app.tab.FocusDown()
}

type KeySeqFunc func(seq string)

func (a *DebuggerApp) InitB() {
	codeWidgetLeft := widgets.NewCodeWidget()
	codeWidgetRight := widgets.NewCodeWidget()

	a.tab = termui.NewTabTwoHozSplitWins(
		"basic debugger",
		codeWidgetLeft,
		codeWidgetRight,
	)
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

}

func (a *DebuggerApp) HandleKey(ev *tcell.EventKey) {

	switch a.appState.Mode() {

	case cgdb.ModeNormal:

		if ev.Key() == tcell.KeyRune && ev.Rune() == ':' {
			a.appState.SetMode(cgdb.ModeCommand)
			a.cmdWidget.Activate()
			return
		} else {
			a.trie.SearchPartial(ev)
			a.tab.HandleEvent(ev)
		}

	case cgdb.ModeCommand:
		a.cmdWidget.HandleEvent(ev)
		if ev.Key() == tcell.KeyEnter {
			a.appState.SetMode(cgdb.ModeNormal)
			a.cmdWidget.Deativate()
		}

	}
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
		if app.appState.Mode() == cgdb.ModeCommand {
			app.appState.SetMode(cgdb.ModeNormal)
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
		tab.HorizontalSplit(widgets.NewCodeWidget())
		app.RequestRedraw()
	}

}

func main() {
	app := NewDebuggerApp()
	app.Run()
}
