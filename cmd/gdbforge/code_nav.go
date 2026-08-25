package main

import (
	"context"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
)

// activeCodeWidget returns the CodeWidget buffer Esc / global keys should drive.
func (a *DebuggerApp) activeCodeWidget() *widgets.CodeWidget {
	if cw := a.focusedCode(); cw != nil {
		return cw
	}
	if path := a.Debug().CurrentFile(); path != "" {
		if w, _ := a.bufs.ensure(path); w != nil {
			return w
		}
	}
	if pc := a.bufs.Primary(); pc != nil {
		return pc
	}
	for _, w := range a.bufs.Buffers() {
		if w != nil {
			return w
		}
	}
	return a.layoutCodeWidget()
}

// onEscape leaves insert/normal Esc handling. When AppState.EscToCode is set
// (default), focuses the last non-code/non-gdb pane if one was active, else
// the CodeWidget leaf; otherwise only leaves insert → normal.
func (a *DebuggerApp) onEscape() {
	if a.State().EscToCode() {
		a.activateLastOrCodePane()
		return
	}
	if tab := a.Tab(); tab != nil {
		tab.SetInsertActive(false)
	}
	a.SetMode(platform.ModeNormal)
	a.RequestRedraw()
}

// sendGdbExec sends an execution command on the shared debugger PTY.
// GDB: prefer MI -exec-* so stops update Code via *stopped without dumping a
// CLI source listing into the console. Delve: keep CLI (no MI mapping).
func (a *DebuggerApp) sendGdbExec(cmd string) {
	if a.gdbWidget == nil || cmd == "" || a.backend == nil {
		return
	}
	sess := a.GDB()
	if sess == nil {
		return
	}
	sendCmd, marksRunning := a.backend.MapExec(cmd)
	if marksRunning && a.State() != nil {
		a.Debug().SetInferiorRunning(true)
	}
	a.State().WithPTYOwner(platform.PTYOwnerUI, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = sess.WithWrite(ctx, func(pw core.PTYWriter) error {
			return pw.Send(sendCmd)
		})
	})
}
