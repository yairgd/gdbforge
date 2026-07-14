package main

import (
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/commands"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/gdb"
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

	cfg       SessionConfig
	gdbClient *gdb.GDBClient
	gdbWidget *widgets.GDBWidget
}

func NewDebuggerApp(cfg SessionConfig, client *gdb.GDBClient, outputChan <-chan core.GdbOutputMsg) *DebuggerApp {
	dbg := &DebuggerApp{
		cfg:       cfg,
		gdbClient: client,
	}
	dbg.TermApp = termui.NewTermApp()
	dbg.TermApp.Api = dbg
	dbg.commandReg = commands.NewCommandRegistry()
	dbg.InitB(outputChan)
	dbg.HandleResize()
	return dbg
}
