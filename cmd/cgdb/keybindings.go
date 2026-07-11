package main

import (
	"github.com/yairgd/cgdb-go/internal/commands"
)

func (a *DebuggerApp) InitKeyBindings() {
	a.keyBindings = commands.NewKeyBindingRegistry()

	a.keyBindings.Bind(
		commands.NewCommand("move-left", func(args ...any) {
			a.OnFocusLeft()
		}),
		"<C-w>l", "<C-w><Left>",
	)

	a.keyBindings.Bind(
		commands.NewCommand("move-right", func(args ...any) {
			a.OnFocusRight()
		}),
		"<C-w>h", "<C-w><Right>",
	)

	a.keyBindings.Bind(
		commands.NewCommand("move-up", func(args ...any) {
			a.OnFocusUp()
		}),
		"<C-w>k", "<C-w><Up>",
	)

	a.keyBindings.Bind(
		commands.NewCommand("move-down", func(args ...any) {
			a.OnFocusDown()
		}),
		"<C-w>j", "<C-w><Down>",
	)
}
