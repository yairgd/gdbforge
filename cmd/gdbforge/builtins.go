package main

import (
	"fmt"

	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/termui"
)

// initBackend starts gdb or dlv before the TUI initializes the terminal.
func (a *DebuggerApp) initBackend() error {
	extTTY := inferiorTTYFromEnvOrCfg(a.cfg)
	if a.cfg.IsDLV() {
		client, err := dlv.NewClientOpts(a.cfg.GDBPath, a.cfg.GDBArgs, dlv.ClientOptions{InferiorTTY: extTTY})
		if err != nil {
			return err
		}
		a.backend = backend.NewDLV(client)
		return nil
	}
	client, err := gdb.NewGDBClientOpts(a.cfg.GDBPath, a.cfg.GDBArgs, gdb.ClientOptions{InferiorTTY: extTTY})
	if err != nil {
		return err
	}
	a.backend = backend.NewGDB(client)
	return nil
}

// initBuiltins creates singleton shell views and starts the debug session.
func (a *DebuggerApp) initBuiltins() error {
	if a.backend == nil {
		return fmt.Errorf("debugger backend not initialized")
	}
	a.builtins = make(map[string]termui.Widget)

	a.aboutWidget = widgets.NewAboutWidget(version)
	a.registerBuiltin("about", a.aboutWidget)

	a.helpWidget = widgets.NewHelpWidget()
	a.helpWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("help", a.helpWidget)

	a.logoWidget = widgets.NewLogoWidget()

	logWidget := termui.NewLoggerWidget(a.ctx)
	logWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("logger", logWidget)

	return a.DebugSession.init(a)
}

func (a *DebuggerApp) registerBuiltin(name string, w termui.Widget) {
	if a.builtins == nil {
		a.builtins = make(map[string]termui.Widget)
	}
	a.builtins[name] = w
}
