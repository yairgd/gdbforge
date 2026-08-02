package main

import (
	"context"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
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

// --- Workspace policy delegates (hosts / keybindings call these) ---

func (a *DebuggerApp) findCodeLeaf() *termui.Node {
	if a.ws == nil {
		return nil
	}
	return a.ws.findCodeLeaf()
}

func (a *DebuggerApp) rememberCodeLeafFromFocus() {
	if a.ws != nil {
		a.ws.rememberCodeLeafFromFocus()
	}
}

func (a *DebuggerApp) focusedLeaf() *termui.Node {
	if a.ws == nil {
		return nil
	}
	return a.ws.focusedLeaf()
}

func (a *DebuggerApp) isGdbLeaf(leaf *termui.Node) bool {
	if a.ws == nil {
		return false
	}
	return a.ws.isGdbLeaf(leaf)
}

func (a *DebuggerApp) focusIsCodeOrGdb() bool {
	if a.ws == nil {
		return true
	}
	return a.ws.focusIsCodeOrGdb()
}

func (a *DebuggerApp) activateLastOrCodePane() {
	if a.ws != nil {
		a.ws.activateLastOrCodePane()
	}
}

func (a *DebuggerApp) findGdbLeaf() *termui.Node {
	if a.ws == nil {
		return nil
	}
	return a.ws.findGdbLeaf()
}

func (a *DebuggerApp) activateGdbPane() {
	if a.ws != nil {
		a.ws.activateGdbPane()
	}
}

func (a *DebuggerApp) activateGdbInsertMode() {
	if a.ws != nil {
		a.ws.activateGdbInsertMode()
	}
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

// FocusCode leaves insert mode and focuses the code slot (Logo/Code/Asm).
func (a *DebuggerApp) FocusCode() {
	if a.ws != nil {
		a.ws.FocusCode()
	}
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
