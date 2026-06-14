package main

import (
	"github.com/yairgd/promptcore/internal/termui"
)

type DebuggerApp struct {
	*termui.TermApp
}

func NewDebuggerApp() *DebuggerApp {
	a := &DebuggerApp{
		TermApp: termui.NewTermApp(),
	}

	a.InitB()

	return a
}

func (a *DebuggerApp) InitB() {
	a.AddWidget(termui.NewCmdWidget())

	codeWidget := termui.NewCodeWidget()
	codeWidget1 := termui.NewCodeWidget()

	a.AddWidget(
		termui.NewTabTwoHozSplitWins(
			a.UpdateCanvas(),
			"basic debugger",
			codeWidget1,
			codeWidget,
		),
	)
}

func main() {
	app := NewDebuggerApp()
	app.Run()
}
