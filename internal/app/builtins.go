package app

import (
	"fmt"

	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/pydbg"
	"github.com/yairgd/termforge"
)

// initBackend starts gdb or dlv before the TUI initializes the terminal.
func (a *DebuggerApp) initBackend() error {
	extTTY := inferiorTTYFromEnvOrCfg(a.cfg)
	if a.cfg.IsPY() {
		client, err := pydbg.NewClient(a.cfg.GDBPath, a.cfg.GDBArgs, pydbg.Options{InferiorTTY: extTTY})
		if err != nil {
			return err
		}
		a.backend = backend.NewPY(client)
		return nil
	}
	if a.cfg.IsDLV() {
		client, err := newDlvClient(a.cfg, extTTY)
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

// newDlvClient attaches to a running headless dlv (-connect) or runs dlv exec.
// With neither a program nor -connect it returns nil: the session starts idle
// until dlv_connect (e.g. :lua remotedlv).
func newDlvClient(cfg SessionConfig, extTTY string) (*dlv.Client, error) {
	if cfg.Connect == "" && len(cfg.GDBArgs) == 0 {
		return nil, nil
	}
	if cfg.Connect != "" {
		return dlv.NewConnectClient(cfg.GDBPath, cfg.Connect)
	}
	return dlv.NewClientOpts(cfg.GDBPath, cfg.GDBArgs, dlv.ClientOptions{InferiorTTY: extTTY})
}

// initBuiltins creates singleton shell views and starts the debug session.
func (a *DebuggerApp) initBuiltins() error {
	if a.backend == nil {
		return fmt.Errorf("debugger backend not initialized")
	}
	a.builtins = make(map[string]termforge.NodeWidget)

	a.aboutWidget = widgets.NewAboutWidget(a.version)
	a.registerBuiltin("about", a.aboutWidget)

	a.helpWidget = widgets.NewHelpWidget()
	a.helpWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("help", a.helpWidget)

	a.logoWidget = widgets.NewLogoWidget()

	logWidget := termforge.NewLoggerWidget(a.ctx)
	logWidget.SetClipboard(a.ClipboardIO())
	a.registerBuiltin("logger", logWidget)

	return a.DebugSession.init(a)
}

func (a *DebuggerApp) registerBuiltin(name string, w termforge.NodeWidget) {
	if a.builtins == nil {
		a.builtins = make(map[string]termforge.NodeWidget)
	}
	a.builtins[name] = w
}
