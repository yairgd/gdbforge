package main

import (
	"log"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/termui"
)

const (
	cmdBreak termui.CommandID = iota + 1
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
	trie termui.Trie
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
	return dbg
}
func (a *DebuggerApp) OnCtrlW(args ...any) {
	seq := args[0].(string)
	keySeq := args[1].([]termui.Key)
	log.Print(keySeq[0].Key)

	log.Print(seq + "AAA")
}

func (app *DebuggerApp) OnFocusLeft(args ...any) {
	w := app.Widgets()[0].Widget()
	tab, _ := (*w).(*termui.TabWidget)
	tab.FocusLeft()

}

func (app *DebuggerApp) OnFocusRight(args ...any) {
	w := app.Widgets()[0].Widget()
	tab, _ := (*w).(*termui.TabWidget)
	tab.FocusRight()

}

func (app *DebuggerApp) OnFocusUp(args ...any) {
	w := app.Widgets()[0].Widget()
	tab, _ := (*w).(*termui.TabWidget)
	tab.FocusUp()

}

func (app *DebuggerApp) OnFocusDown(args ...any) {
	w := app.Widgets()[0].Widget()
	tab, _ := (*w).(*termui.TabWidget)
	tab.FocusDown()

}

type KeySeqFunc func(seq string)

func (a *DebuggerApp) InitB() {
	codeWidgetLeft := widgets.NewCodeWidget()
	codeWidgetRight := widgets.NewCodeWidget()

	a.AddWidget(
		termui.NewTabTwoHozSplitWins(
			"basic debugger",
			codeWidgetLeft,
			codeWidgetRight,
		),
	)

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
	cmd := termui.NewCmdWidget(completer)
	cmd.Events = a.Events()
	a.AddWidget(cmd)

	a.BindKeySeq(a.OnFocusLeft, "<C-w>l", "<C-w><Left>")
	a.BindKeySeq(a.OnFocusRight, "<C-w>h", "<C-w><Right>")
	a.BindKeySeq(a.OnFocusUp, "<C-w>k", "<C-w><Up>")
	a.BindKeySeq(a.OnFocusDown, "<C-w>j", "<C-w><Down>")

}

func (a *DebuggerApp) HandleUIEvent(ev tcell.Event) {
	switch ev.(type) {
	case *tcell.EventKey:
		a.trie.SearchPartial(ev)

	case *tcell.EventResize:
		c := a.UpdateCanvas()
		w := a.Widgets()
		if len(w) < 2 {
			return
		}
		w[0].SetRect(c.ChildRect(0, 0, c.W(), c.H()))
		w[1].SetRect(c.ChildRect(0, c.H()-1, c.W(), 1))
	}
}

func (app *DebuggerApp) HandleCoreEvents(ev termui.Event) {
	msg, ok := ev.(termui.CommandEvent)
	if !ok {
		return
	}

	switch msg.CommandID() {
	case termui.CmdUnknown:
		// TODO: show unknown command feedback in the UI
	case cmdQuit:
		app.Exit()
	case cmdVerticalSplit:
		w := app.Widgets()[0].Widget()
		tab, ok := (*w).(*termui.TabWidget)
		if !ok {
			return
		}
		tab.VerticalSplit(widgets.NewCodeWidget())
		app.RequestRedraw()

	case cmdHorizontalSplit:
		w := app.Widgets()[0].Widget()
		tab, ok := (*w).(*termui.TabWidget)
		if !ok {
			return
		}
		tab.HorizontalSplit(widgets.NewCodeWidget())
		app.RequestRedraw()
	}

}

func f1(args ...any) {
	x := args[0].(int)
	y := args[1].(int)

	log.Print(x * y)
}

func main() {
	app := NewDebuggerApp()
	app.Run()
}
