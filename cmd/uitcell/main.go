package main

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/promptcore/internal/termui"
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

	// Command line widget at the bottom.
	a.AddWidget(termui.NewCmdWidget())
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

func main() {

	// Create debugger application.
	app := NewDebuggerApp()

	// Start TUI event loop.
	app.Run()

	// app.Screen().Fini()
}
