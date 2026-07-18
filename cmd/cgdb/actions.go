package main

import (
	"context"
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
	"github.com/yairgd/cgdb-go/internal/execcli"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

func (app *DebuggerApp) OnFocusLeft(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send left command")
	app.tab.FocusLeft()
}

func (app *DebuggerApp) OnFocusRight(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send right command")

	app.tab.FocusRight()
}

func (app *DebuggerApp) OnFocusUp(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send up command")

	app.tab.FocusUp()
}

func (app *DebuggerApp) OnFocusDown(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send down command")

	app.tab.FocusDown()
}

func (app *DebuggerApp) BreakFile(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("break file")
}

func (app *DebuggerApp) DeleteBreakpoint(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("delete breakpoint")
}

func (app *DebuggerApp) ShowRegisters(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("show registers")
}

func (app *DebuggerApp) ShowThreads(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("show threads")
}

func (app *DebuggerApp) SplitHorizontal(args ...any) {
	w := app.Widgets()[0].Widget()
	tab, ok := w.(*termui.TabWidget)
	if !ok {
		return
	}

	l := termui.NewLoggerWidget(app.ctx)
	l.Events = app.Events()
	l.SetClipboard(app.ClipboardIO())
	tab.HorizontalSplit(l)
	app.RequestRedraw()
}

func (app *DebuggerApp) SplitVertical(args ...any) {
	w := widgets.NewCodeWidget()
	w.PaneName = "[No Name]"
	w.SetClipboard(app.ClipboardIO())
	if app.gdbWidget != nil {
		w.SetPTY(app.gdbWidget.Session(), app.State())
	}
	app.tab.VerticalSplit(w)
	app.RequestRedraw()
}

func (app *DebuggerApp) EnterInsertMode(args ...any) {
	app.tab.SetInsertActive(true)
	app.SetMode(platform.ModeInsert)
	app.RequestRedraw()
}

func (app *DebuggerApp) ClearFocus(args ...any) {
	w := app.tab.FocusedWidget()
	if c, ok := w.(termui.Clearable); ok {
		c.Clear()
	}
	app.RequestFrame()
}

func (app *DebuggerApp) Quit(args ...any) {
	if app.tab.DeleteFocus() {
		app.Close()
		app.Exit()
	}
	app.RequestRedraw()
}

func (app *DebuggerApp) SetEqualAlwaysOn(args ...any) {
	app.State().SetEqualAlways(true)
	if app.tab != nil {
		app.tab.SetEqualAlways(true)
		if tree := app.tab.ActiveTree(); tree != nil {
			tree.Rebalance()
		}
	}
	app.RequestFrame()
}

func (app *DebuggerApp) SetEqualAlwaysOff(args ...any) {
	app.State().SetEqualAlways(false)
	if app.tab != nil {
		app.tab.SetEqualAlways(false)
	}
	app.RequestFrame()
}

func (app *DebuggerApp) SetClearOutputOn(args ...any) {
	app.State().SetClearOutput(true)
}

func (app *DebuggerApp) SetClearOutputOff(args ...any) {
	app.State().SetClearOutput(false)
}

// OnAI runs an in-app LLM question against the live GDB session (:AI … / :ai …).
func (app *DebuggerApp) OnAI(args ...any) {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		s, ok := a.(string)
		if !ok || s == "" {
			continue
		}
		parts = append(parts, s)
	}
	question := strings.TrimSpace(strings.Join(parts, " "))
	if question == "" || app.gdbMcp == nil {
		return
	}

	log := app.ctx.Log.Named("ai")
	if app.gdbWidget != nil {
		app.gdbWidget.AppendLines([]string{">>> AI: " + question})
		app.RequestFrame()
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		answer, err := app.gdbMcp.Ask(ctx, question)
		if err != nil {
			log.Error(err.Error())
			answer = "AI error: " + err.Error()
		}
		lines := []string{">>> AI reply:"}
		for _, line := range strings.Split(answer, "\n") {
			lines = append(lines, line)
		}
		_ = app.Screen().PostEvent(tcell.NewEventInterrupt(aiReplyMsg{lines: lines}))
	}()
}

type aiReplyMsg struct {
	lines []string
}

// OnRun starts (or restarts) an ExecClient for the given argv and shows ExecWidget
// in the focused pane. Example: :!bash  or  :!ssh root@192.168.20.50
func (app *DebuggerApp) OnRun(args ...any) {
	argv := make([]string, 0, len(args))
	for _, a := range args {
		s, ok := a.(string)
		if !ok || s == "" {
			continue
		}
		argv = append(argv, s)
	}
	if len(argv) == 0 {
		return
	}

	if app.execClient != nil {
		app.execClient.Close()
		app.execClient = nil
	}

	client, err := execcli.NewExecClient(argv)
	if err != nil {
		if app.ctx.Log != nil {
			app.ctx.Log.Named("exec").Error(err.Error())
		}
		return
	}
	app.execClient = client

	w := widgets.NewExecWidget(client)
	w.SetClipboard(app.ClipboardIO())
	w.SetSizeFunc(client.SetSize)
	w.SetOnClose(func() {
		client.Close()
	})
	ch, _ := client.Subscribe()
	w.StartExecUIBridge(app.Screen(), ch)
	app.execWidget = w
	app.registerBuiltin("exec", w)

	if app.tab != nil && app.swapFocusedWidget(w) {
		app.EnterInsertMode()
		app.RequestFrame()
	}
}
