package main

import (
	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
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
		commands.NewCommand("only-window", func(args ...any) { a.OnlyFocus() }),
		"<C-w>o", "<C-w><C-o>",
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
		commands.NewCommand("command-mode", func(args ...any) { a.enterCommandMode() }),
		":",
	)
	a.keyBindings.Bind(
		commands.NewCommand("search-mode", func(args ...any) { a.enterSearchMode() }),
		"/",
	)
	a.keyBindings.Bind(
		commands.NewCommand("search-word-next", func(args ...any) { a.searchWordMatch(1) }),
		"*",
	)
	a.keyBindings.Bind(
		commands.NewCommand("search-word-prev", func(args ...any) { a.searchWordMatch(-1) }),
		"#",
	)
	a.keyBindings.Bind(
		commands.NewCommand("insert-gdb", func(args ...any) { a.activateGdbInsertMode() }),
		"i",
	)

	a.keyBindings.Bind(
		commands.NewCommand("gdb-quit", func(args ...any) {
			if gb := a.gdbBackend(); gb != nil && gb.Client != nil {
				a.console.handleGdbQuitAction(gb.Client.RequestQuit(), "q")
			} else if a.isDLV() {
				a.console.onGdbConsoleEOF()
			}
			a.RequestFrame()
		}),
		"<C-d>",
	)
	// Ctrl-Z is handled globally (withGlobalKeys) in every mode — not only normal.

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
		commands.NewHandledCommand("code-page-up", a.tryCodePageUp),
		"<PgUp>", "<C-b>",
	)
	a.keyBindings.Bind(
		commands.NewHandledCommand("code-page-down", a.tryCodePageDown),
		"<PgDn>", "<C-f>",
	)
	a.keyBindings.Bind(
		commands.NewHandledCommand("code-left", a.tryCodeMoveLeft),
		"<Left>",
	)
	a.keyBindings.Bind(
		commands.NewHandledCommand("code-right", a.tryCodeMoveRight),
		"<Right>",
	)
	a.keyBindings.Bind(
		commands.NewHandledCommand("space-break", a.trySpaceBreak),
		" ",
	)
	a.keyBindings.Bind(
		commands.NewHandledCommand("code-toggle-enable", a.tryCodeToggleEnable),
		"e",
	)
	// n: next search match when a pattern is active, else GDB next.
	a.keyBindings.Bind(
		commands.NewHandledCommand("search-or-gdb-next", a.trySearchOrGdbNext),
		"n",
	)
	a.keyBindings.Bind(
		commands.NewCommand("search-prev-match", func(args ...any) { a.searchPrevMatch() }),
		"N",
	)
	a.keyBindings.Bind(
		commands.NewCommand("gdb-step", func(args ...any) { a.sendGdbExec("step") }),
		"s",
	)
	a.keyBindings.Bind(
		commands.NewCommand("gdb-continue", func(args ...any) { a.sendGdbExec("continue") }),
		"c",
	)
	a.keyBindings.Bind(
		commands.NewCommand("gdb-finish", func(args ...any) { a.sendGdbExec("finish") }),
		"f",
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
		commands.NewHandledCommand("code-continue", a.tryInsertCodeContinue),
		"c",
	)
	a.insertKeys.Bind(
		commands.NewHandledCommand("code-finish", a.tryInsertCodeFinish),
		"f",
	)
	a.insertKeys.Bind(
		commands.NewHandledCommand("space-break", a.trySpaceBreak),
		" ",
	)
}

func (a *DebuggerApp) initCompletionKeyBindings() {
	a.completionKeys.Bind(
		commands.NewCommand("cancel", func(args ...any) {
			a.leaveCompletionMode()
			a.RequestFrame()
		}),
		"<Esc>",
	)
	a.completionKeys.Bind(
		commands.NewCommand("accept", func(args ...any) {
			if a.completionMenu != nil {
				if name := a.completionMenu.Selected(); name != "" {
					useGDB := a.completionForGDB && a.gdbWidget != nil &&
						(a.cmdWidget == nil || !a.cmdWidget.Active())
					if useGDB {
						cur := a.gdbWidget.InputText()
						a.gdbWidget.ApplyCompletion(gdb.WithCompletionSpace(gdb.ApplyMenuChoice(cur, name)))
					} else if a.cmdWidget != nil {
						a.cmdWidget.ApplyCompletion(name)
					}
				}
			}
			a.leaveCompletionMode()
			a.RequestFrame()
		}),
		"<Enter>",
	)
	a.completionKeys.Bind(
		commands.NewCommand("prev", func(args ...any) {
			if a.completionMenu != nil {
				a.completionMenu.Move(-1)
				a.syncCompletionView()
			}
			a.RequestFrame()
		}),
		"<Left>", "<Up>",
	)
	a.completionKeys.Bind(
		commands.NewCommand("next", func(args ...any) {
			if a.completionMenu != nil {
				a.completionMenu.Move(1)
				a.syncCompletionView()
			}
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
	return a.tryCodeDelta(-1, 0)
}

func (a *DebuggerApp) tryCodeMoveDown() bool {
	return a.tryCodeDelta(1, 0)
}

func (a *DebuggerApp) tryCodePageUp() bool {
	return a.tryCodePage(-1)
}

func (a *DebuggerApp) tryCodePageDown() bool {
	return a.tryCodePage(1)
}

func (a *DebuggerApp) tryCodeMoveLeft() bool {
	return a.tryCodeCol(-1)
}

func (a *DebuggerApp) tryCodeMoveRight() bool {
	return a.tryCodeCol(1)
}

func (a *DebuggerApp) tryCodeDelta(lineDelta, _ int) bool {
	if !a.focusIsCodeOrGdb() {
		return false
	}
	if aw, ok := a.focusedWidget().(*widgets.AssemblyWidget); ok && aw != nil {
		aw.MoveSel(lineDelta)
		a.RequestFrame()
		return true
	}
	if cw := a.activeCodeWidget(); cw != nil {
		cw.MoveSel(lineDelta)
		a.RequestFrame()
		return true
	}
	return false
}

func (a *DebuggerApp) tryCodePage(dir int) bool {
	if !a.focusIsCodeOrGdb() {
		return false
	}
	if aw, ok := a.focusedWidget().(*widgets.AssemblyWidget); ok && aw != nil {
		n := aw.VisibleRows()
		if n < 1 {
			n = 10
		}
		aw.MoveSel(dir * n)
		a.RequestFrame()
		return true
	}
	if cw := a.activeCodeWidget(); cw != nil {
		cw.MoveSel(dir * 10)
		a.RequestFrame()
		return true
	}
	return false
}

func (a *DebuggerApp) tryCodeCol(delta int) bool {
	if !a.focusIsCodeOrGdb() {
		return false
	}
	if cw := a.activeCodeWidget(); cw != nil {
		cw.MoveCol(delta)
		a.RequestFrame()
		return true
	}
	return false
}

func (a *DebuggerApp) tryCodeBreakAtSel() bool {
	if !a.focusIsCodeOrGdb() {
		return false
	}
	if aw, ok := a.focusedWidget().(*widgets.AssemblyWidget); ok && aw != nil {
		aw.BreakAtSel()
		a.RequestFrame()
		return true
	}
	if cw := a.activeCodeWidget(); cw != nil {
		cw.BreakAtSel()
		a.RequestFrame()
		return true
	}
	return false
}

// trySpaceBreak toggles a breakpoint: Call Stack selection, or Code/Asm cursor
// (same Space behavior as CodeWidget). Falls through for other panes / GDB typing.
func (a *DebuggerApp) trySpaceBreak() bool {
	if a.focusedIsCallstack() {
		a.toggleCallstackBreak()
		return true
	}
	// Never steal Space from the GDB console (insert typing).
	if a.focusedIsGdb() {
		return false
	}
	return a.tryCodeBreakAtSel()
}

func (a *DebuggerApp) tryCodeToggleEnable() bool {
	if a.focusedIsBreakpoint() {
		return false
	}
	if aw, ok := a.focusedWidget().(*widgets.AssemblyWidget); ok && aw != nil {
		a.ToggleAsmBreakEnable()
		a.RequestFrame()
		return true
	}
	if cw := a.activeCodeWidget(); cw != nil {
		if focused := a.focusedCode(); focused != nil {
			cw = focused
		}
		a.breaks.toggleCodeBreakEnableOn(cw)
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

func (a *DebuggerApp) tryInsertCodeContinue() bool {
	if !a.focusedIsCode() {
		return false
	}
	a.sendGdbExec("continue")
	return true
}

func (a *DebuggerApp) tryInsertCodeFinish() bool {
	if !a.focusedIsCode() {
		return false
	}
	a.sendGdbExec("finish")
	return true
}

// toggleCallstackBreak inserts or clears a breakpoint at the selected frame
// (same path as CodeWidget Space — includes DLV cmd mapping).
func (a *DebuggerApp) toggleCallstackBreak() {
	cs := a.focusedCallstack()
	if cs == nil {
		return
	}
	fr, ok := cs.SelectedFrame()
	if !ok || fr.File == "" || fr.Line < 1 {
		return
	}
	a.breaks.onCodeBreakToggle(fr.File, fr.Line)
	a.RequestFrame()
}

func (a *DebuggerApp) hasBreakAt(file string, line int) bool {
	if a.breakpoints == nil || file == "" || line < 1 {
		return false
	}
	return a.breakpoints.IndexOfFileLine(file, line) >= 0
}
