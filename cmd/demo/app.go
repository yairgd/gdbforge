package main

import (
	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/demo"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// DemoApp is a host-only showcase with gdbforge-like chrome and basic commands.
type DemoApp struct {
	*termui.TermApp
	commandReg  *commands.CommandRegistry
	keyBindings *commands.KeyBindingRegistry
	insertKeys  *commands.KeyBindingRegistry

	tab       *termui.TabWidget
	cmdWidget *termui.CmdWidget
	ctx       platform.AppContext

	mainPane *demo.ScrollPane
	sidePane *demo.ScrollPane
	logPane  *termui.LoggerWidget
	builtins map[string]termui.Widget
}

// NewDemoApp builds and initializes the demo TUI.
func NewDemoApp() (*DemoApp, error) {
	a := &DemoApp{
		TermApp:    termui.NewTermApp(),
		commandReg: commands.NewCommandRegistry(),
		builtins:   make(map[string]termui.Widget),
	}
	a.TermApp.Api = a
	if err := a.Init(); err != nil {
		a.Close()
		return nil, err
	}
	a.HandleResize()
	return a, nil
}

// Close releases app resources.
func (a *DemoApp) Close() {
	if a.TermApp != nil {
		a.TermApp.Close()
	}
}
