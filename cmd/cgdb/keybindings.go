package main

import (
	"fmt"
	"path/filepath"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/commands"
	"github.com/yairgd/cgdb-go/internal/platform"
)

func (a *DebuggerApp) InitKeyBindings() {
	a.keyBindings = commands.NewKeyBindingRegistry()
	a.insertKeys = commands.NewKeyBindingRegistry()
	a.completionKeys = commands.NewKeyBindingRegistry()

	a.initNormalKeyBindings()
	a.initInsertKeyBindings()
	a.initCompletionKeyBindings()
}

func (a *DebuggerApp) initNormalKeyBindings() {
	a.keyBindings.Bind(
		commands.NewCommand("move-left", func(args ...any) { a.OnFocusLeft() }),
		"<C-w>l", "<C-w><Left>",
	)
	a.keyBindings.Bind(
		commands.NewCommand("move-right", func(args ...any) { a.OnFocusRight() }),
		"<C-w>h", "<C-w><Right>",
	)
	a.keyBindings.Bind(
		commands.NewCommand("move-up", func(args ...any) { a.OnFocusUp() }),
		"<C-w>k", "<C-w><Up>",
	)
	a.keyBindings.Bind(
		commands.NewCommand("move-down", func(args ...any) { a.OnFocusDown() }),
		"<C-w>j", "<C-w><Down>",
	)
	a.keyBindings.Bind(
		commands.NewCommand("jump-back", a.JumpBack),
		"<C-o>",
	)

	a.keyBindings.Bind(
		commands.NewCommand("escape", func(args ...any) { a.onEscape() }),
		"<Esc>",
	)
	a.keyBindings.Bind(
		commands.NewCommand("command-mode", func(args ...any) {
			if a.completionBar != nil {
				a.completionBar.Clear()
			}
			a.SetMode(platform.ModeCommand)
			a.cmdWidget.Activate()
		}),
		":",
	)
	a.keyBindings.Bind(
		commands.NewCommand("insert-gdb", func(args ...any) { a.activateGdbInsertMode() }),
		"i",
	)

	// Code globals (gated fallthrough where list panes own the key).
	a.keyBindings.Bind(
		commands.NewHandledCommand("code-up", a.tryCodeMoveUp),
		"<Up>",
	)
	a.keyBindings.Bind(
		commands.NewHandledCommand("code-down", a.tryCodeMoveDown),
		"<Down>",
	)
	a.keyBindings.Bind(
		commands.NewHandledCommand("space-break", a.trySpaceBreak),
		" ",
	)
	a.keyBindings.Bind(
		commands.NewHandledCommand("code-toggle-enable", a.tryCodeToggleEnable),
		"e",
	)
	a.keyBindings.Bind(
		commands.NewCommand("gdb-next", func(args ...any) { a.sendGdbExec("next") }),
		"n",
	)
	a.keyBindings.Bind(
		commands.NewCommand("gdb-step", func(args ...any) { a.sendGdbExec("step") }),
		"s",
	)
}

func (a *DebuggerApp) initInsertKeyBindings() {
	a.insertKeys.Bind(
		commands.NewCommand("escape", func(args ...any) { a.onEscape() }),
		"<Esc>",
	)
	a.insertKeys.Bind(
		commands.NewHandledCommand("code-next", a.tryInsertCodeNext),
		"n",
	)
	a.insertKeys.Bind(
		commands.NewHandledCommand("code-step", a.tryInsertCodeStep),
		"s",
	)
	a.insertKeys.Bind(
		commands.NewHandledCommand("space-break", a.trySpaceBreak),
		" ",
	)
}

func (a *DebuggerApp) initCompletionKeyBindings() {
	a.completionKeys.Bind(
		commands.NewCommand("cancel", func(args ...any) {
			a.completionBar.Clear()
			a.SetMode(platform.ModeCommand)
			a.RequestFrame()
		}),
		"<Esc>",
	)
	a.completionKeys.Bind(
		commands.NewCommand("accept", func(args ...any) {
			if name := a.completionBar.Selected(); name != "" {
				a.cmdWidget.ApplyCompletion(name)
			}
			a.completionBar.Clear()
			a.SetMode(platform.ModeCommand)
			a.RequestFrame()
		}),
		"<Enter>",
	)
	a.completionKeys.Bind(
		commands.NewCommand("prev", func(args ...any) {
			a.completionBar.MoveSelection(-1)
			a.RequestFrame()
		}),
		"<Left>", "<Up>",
	)
	a.completionKeys.Bind(
		commands.NewCommand("next", func(args ...any) {
			a.completionBar.MoveSelection(1)
			a.RequestFrame()
		}),
		"<Right>", "<Down>", "<Tab>",
	)
}

// tryKeyBindings runs a mode key trie. Returns true if the key was consumed
// (action ran, or unfinished chord). Returns false on miss or declined Handled.
func (a *DebuggerApp) tryKeyBindings(reg *commands.KeyBindingRegistry, ev *tcell.EventKey) bool {
	if reg == nil {
		return false
	}
	key, ok := platform.KeyFromEvent(ev)
	if !ok {
		reg.ResetPartial()
		return false
	}
	completed, handled := reg.HandleKey(key)
	if !handled {
		return false
	}
	if !completed {
		return reg.InPartial()
	}
	return true
}

func (a *DebuggerApp) tryCodeMoveUp() bool {
	if !a.focusIsCodeOrGdb() {
		return false
	}
	if cw := a.activeCodeWidget(); cw != nil {
		cw.MoveSel(-1)
		a.RequestFrame()
		return true
	}
	return false
}

func (a *DebuggerApp) tryCodeMoveDown() bool {
	if !a.focusIsCodeOrGdb() {
		return false
	}
	if cw := a.activeCodeWidget(); cw != nil {
		cw.MoveSel(1)
		a.RequestFrame()
		return true
	}
	return false
}

func (a *DebuggerApp) tryCodeBreakAtSel() bool {
	if !a.focusIsCodeOrGdb() {
		return false
	}
	if cw := a.activeCodeWidget(); cw != nil {
		cw.BreakAtSel()
		a.RequestFrame()
		return true
	}
	return false
}

// trySpaceBreak toggles a breakpoint: Call Stack selection, or Code cursor
// (same Space behavior as CodeWidget). Falls through for other panes.
func (a *DebuggerApp) trySpaceBreak() bool {
	if a.focusedIsCallstack() {
		a.toggleCallstackBreak()
		return true
	}
	return a.tryCodeBreakAtSel()
}

func (a *DebuggerApp) tryCodeToggleEnable() bool {
	if a.focusedIsBreakpoint() {
		return false
	}
	if cw := a.activeCodeWidget(); cw != nil {
		if focused := a.focusedCode(); focused != nil {
			cw = focused
		}
		a.toggleCodeBreakEnableOn(cw)
	}
	return true
}

func (a *DebuggerApp) tryInsertCodeNext() bool {
	if !a.focusedIsCode() {
		return false
	}
	a.sendGdbExec("next")
	return true
}

func (a *DebuggerApp) tryInsertCodeStep() bool {
	if !a.focusedIsCode() {
		return false
	}
	a.sendGdbExec("step")
	return true
}

// toggleCallstackBreak inserts or clears a breakpoint at the selected frame
// (same toggle semantics as CodeWidget Space).
func (a *DebuggerApp) toggleCallstackBreak() {
	cs := a.focusedCallstack()
	if cs == nil {
		return
	}
	fr, ok := cs.SelectedFrame()
	if !ok || fr.File == "" || fr.Line < 1 {
		return
	}
	if a.gdbWidget == nil {
		return
	}
	sess := a.gdbWidget.Session()
	if sess == nil {
		return
	}
	loc := fmt.Sprintf("%s:%d", filepath.Base(fr.File), fr.Line)
	cmd := "break " + loc
	if a.hasBreakAt(fr.File, fr.Line) {
		cmd = "clear " + loc
	}
	widgets.SendGdbCmd(sess, a.State(), cmd)
	a.onBreakpointsChanged()
	a.RequestFrame()
}

func (a *DebuggerApp) hasBreakAt(file string, line int) bool {
	if a.bpWidget == nil || file == "" || line < 1 {
		return false
	}
	base := filepath.Base(file)
	for _, it := range a.bpWidget.Items() {
		if it.Line != line {
			continue
		}
		if it.File == file || filepath.Base(it.File) == base {
			return true
		}
	}
	return false
}
