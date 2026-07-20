package main

import (
	tcell "github.com/gdamore/tcell/v2"
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
		commands.NewHandledCommand("code-break", a.tryCodeBreakAtSel),
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
