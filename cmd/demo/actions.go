package main

import (
	"strings"

	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/demo"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

func (a *DemoApp) ExapData() {
	a.commandReg.Root.
		Group("window",
			commands.Cmd("left", a.OnFocusLeft),
			commands.Cmd("right", a.OnFocusRight),
			commands.Cmd("up", a.OnFocusUp),
			commands.Cmd("down", a.OnFocusDown),
		).
		LeafRestComplete("b", a.OnBuffer, a.bufferCompletions).
		Leaf("help", a.OnHelp).
		Leaf("vs", a.SplitVertical).
		Leaf("split", a.SplitHorizontal).
		Leaf("clear", a.ClearFocus).
		Leaf("quit", a.Quit)
}

func (a *DemoApp) bufferCompletions(prefix string) []string {
	names := []string{"main", "side", "log", "help"}
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	var out []string
	for _, n := range names {
		if prefix == "" || strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out
}

func (a *DemoApp) OnFocusLeft(args ...any) {
	if a.tab != nil {
		a.tab.FocusLeft()
		a.RequestFrame()
	}
}
func (a *DemoApp) OnFocusRight(args ...any) {
	if a.tab != nil {
		a.tab.FocusRight()
		a.RequestFrame()
	}
}
func (a *DemoApp) OnFocusUp(args ...any) {
	if a.tab != nil {
		a.tab.FocusUp()
		a.RequestFrame()
	}
}
func (a *DemoApp) OnFocusDown(args ...any) {
	if a.tab != nil {
		a.tab.FocusDown()
		a.RequestFrame()
	}
}

func (a *DemoApp) EnterInsertMode(args ...any) {
	if a.tab != nil {
		a.tab.SetInsertActive(true)
	}
	a.SetMode(platform.ModeInsert)
	a.RequestRedraw()
}

func (a *DemoApp) SplitVertical(args ...any) {
	p := termui.NewConsolePane("split")
	p.SetClipboard(a.ClipboardIO())
	p.Buffer().AppendLine("[split]")
	a.tab.VerticalSplit(p)
	a.RequestRedraw()
}

func (a *DemoApp) SplitHorizontal(args ...any) {
	p := termui.NewConsolePane("split")
	p.SetClipboard(a.ClipboardIO())
	p.Buffer().AppendLine("[split]")
	a.tab.HorizontalSplit(p)
	a.RequestRedraw()
}

func (a *DemoApp) ClearFocus(args ...any) {
	w := a.focusedWidget()
	if c, ok := w.(termui.Clearable); ok {
		c.Clear()
	}
	a.RequestFrame()
}

func (a *DemoApp) OnHelp(args ...any) {
	if a.mainPane == nil {
		return
	}
	a.mainPane.Clear()
	for _, line := range strings.Split(demo.HelpText, "\n") {
		a.mainPane.Buffer().AppendLine(line)
	}
	a.tab.FocusWidget(a.mainPane)
	a.RequestFrame()
}

func (a *DemoApp) OnBuffer(args ...any) {
	name := ""
	if len(args) > 0 {
		if s, ok := args[0].(string); ok {
			name = strings.TrimSpace(s)
		}
	}
	if name == "" || name == "help" {
		a.OnHelp()
		return
	}
	w, ok := a.builtins[name]
	if !ok || a.tab == nil {
		a.ctx.Log.Named("demo").Warn("unknown buffer: " + name)
		return
	}
	a.tab.FocusWidget(w)
	a.RequestFrame()
}

func (a *DemoApp) Quit(args ...any) {
	if a.tab != nil && a.tab.DeleteFocus() {
		a.Exit()
	} else {
		a.Exit()
	}
}
