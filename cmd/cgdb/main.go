package main

import (
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
)

type DebuggerApp struct {
	*termui.TermApp
}

func NewDebuggerApp() *DebuggerApp {
	dbg := &DebuggerApp{}
	dbg.TermApp = termui.NewTermApp()
	dbg.TermApp.Api = dbg
	dbg.InitB()
	return dbg
}

func (a *DebuggerApp) InitB() {
	codeWidgetLeft := widgets.NewCodeWidget()
	codeWidgetRight := widgets.NewCodeWidget()

	c := a.UpdateCanvas()

	a.AddWidget(
		termui.NewTabTwoHozSplitWins(
			c,
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
	})
	cmd := termui.NewCmdWidget(completer)
	cmd.Events = a.Events()
	a.AddWidget(cmd)
}

func (a *DebuggerApp) HandleUIEvent(ev tcell.Event) {
	switch ev.(type) {
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
	}
}

func main() {
	app := NewDebuggerApp()
	app.Run()
}
