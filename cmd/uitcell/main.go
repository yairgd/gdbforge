package main

import (
	"github.com/yairgd/promptcore/internal/termui"
)

type DebuggerApp struct {
}

func (a *DebuggerApp) Init() {

	// a.A
}

func main() {
	app := termui.NewTermApp()
	//	app.AddWidget(termui.NewCmdWidget())

	codeWidget := termui.NewCodeWidget()
	codeWidget1 := termui.NewCodeWidget()
	app.AddWidget(termui.NewTabTwoHozSplitWins(app.UpdateCanvas(), "basic debuger", codeWidget1, codeWidget))

	app.Run()

}

const debug = false
