package main

import (
	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/platform"
)

func (a *DemoApp) InitKeyBindings() {
	a.keyBindings = commands.NewKeyBindingRegistry()
	a.insertKeys = commands.NewKeyBindingRegistry()

	a.keyBindings.Bind(
		commands.NewCommand("move-left", func(args ...any) { a.OnFocusLeft() }),
		"<C-w>h", "<C-w><Left>",
	)
	a.keyBindings.Bind(
		commands.NewCommand("move-right", func(args ...any) { a.OnFocusRight() }),
		"<C-w>l", "<C-w><Right>",
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
		commands.NewCommand("escape", func(args ...any) { a.onEscape() }),
		"<Esc>",
	)
	a.keyBindings.Bind(
		commands.NewCommand("command-mode", func(args ...any) { a.enterCommandMode() }),
		":",
	)
	a.keyBindings.Bind(
		commands.NewCommand("insert", func(args ...any) { a.EnterInsertMode() }),
		"i",
	)
	a.keyBindings.Bind(
		commands.NewCommand("quit", func(args ...any) { a.Quit() }),
		"<C-d>",
	)

	a.insertKeys.Bind(
		commands.NewCommand("escape", func(args ...any) { a.onEscape() }),
		"<Esc>",
	)
}

func (a *DemoApp) onEscape() {
	if a.Mode() == platform.ModeCommand {
		a.leaveCommandMode()
		return
	}
	if a.tab != nil {
		a.tab.SetInsertActive(false)
	}
	a.SetMode(platform.ModeNormal)
	a.RequestFrame()
}

func (a *DemoApp) enterCommandMode() {
	if a.tab != nil {
		a.tab.SetInsertActive(false)
	}
	if a.cmdWidget != nil && !a.cmdWidget.Active() {
		a.cmdWidget.Activate()
	}
	a.SetMode(platform.ModeCommand)
	a.RequestFrame()
}

func (a *DemoApp) leaveCommandMode() {
	if a.cmdWidget != nil {
		a.cmdWidget.Deativate()
	}
	a.SetMode(platform.ModeNormal)
	a.RequestFrame()
}
