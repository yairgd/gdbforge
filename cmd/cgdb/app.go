package main

import (
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
