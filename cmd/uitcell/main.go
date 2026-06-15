package main

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/core"
	"github.com/yairgd/promptcore/internal/termui"
)

const (
	cmdBreak core.CommandID = iota + 1
	cmdContinue
	cmdNext
	cmdStep
	cmdPrint
	cmdBacktrace
	cmdInfo
	cmdRun
	cmdQuit
)

// DebuggerApp extends TermApp with debugger-specific behavior.
type DebuggerApp struct {
	*termui.TermApp
}

// Create the debugger application.
func NewDebuggerApp() *DebuggerApp {
	dbg := &DebuggerApp{}

	// Create the base TUI application.
	dbg.TermApp = termui.NewTermApp()

	// Register ourselves as the UI event handler.
	dbg.TermApp.Api = dbg

	// Build the initial widget layout.
	dbg.InitB()

	return dbg
}

// Create all top-level widgets and their initial layout.
func (a *DebuggerApp) InitB() {

	// Two source/code windows that will be displayed
	// inside a tabbed split container.
	codeWidgetLeft := termui.NewCodeWidget()
	codeWidgetRight := termui.NewCodeWidget()

	// Current full-screen canvas.
	c := a.UpdateCanvas()

	// Main debugger area.
	// Occupies the entire screen initially.
	a.AddWidget(
		termui.NewTabTwoHozSplitWins(
			c,
			"basic debugger",
			codeWidgetLeft,
			codeWidgetRight,
		),
	)

	completer := core.NewSimpleCompleter([]core.Command{
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

// Called by TermApp when a UI event occurs.
// We currently use it to recalculate widget geometry
// when the terminal size changes.
func (a *DebuggerApp) HandleUIEvent(ev tcell.Event) {

	switch ev.(type) {
	case *tcell.EventResize:
		// Recalculate root canvas dimensions.
		c := a.UpdateCanvas()

		// Access top-level widgets.
		w := a.Widgets()

		if len(w) < 2 {
			return
		}

		// Main debugger/tab area.
		w[0].SetRect(
			c.ChildRect(
				0,
				0,
				c.W(),
				c.H(), // reserve last line for command widget
			),
		)

		// Bottom command line.
		w[1].SetRect(
			c.ChildRect(
				0,
				c.H()-1,
				c.W(),
				1,
			),
		)
	}
}

func (app *DebuggerApp) HandleCoreEvents(ev core.Event) {
	msg, ok := ev.(core.CommandEvent)
	if !ok {
		return
	}

	switch msg.CommandID() {
	case core.CmdUnknown:
		// TODO: show unknown command feedback in the UI
	case cmdQuit:
		app.Exit()
	}
}

func main() {

	// Create debugger application.
	app := NewDebuggerApp()

	// Start TUI event loop.
	app.Run()
}
