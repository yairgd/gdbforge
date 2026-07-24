package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/execcli"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

func (app *DebuggerApp) OnFocusLeft(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send left command")
	app.tab.FocusLeft()
	app.rememberCodeLeafFromFocus()
}

func (app *DebuggerApp) OnFocusRight(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send right command")

	app.tab.FocusRight()
	app.rememberCodeLeafFromFocus()
}

func (app *DebuggerApp) OnFocusUp(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send up command")

	app.tab.FocusUp()
	app.rememberCodeLeafFromFocus()
}

func (app *DebuggerApp) OnFocusDown(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send down command")

	app.tab.FocusDown()
	app.rememberCodeLeafFromFocus()
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
	app.wireCodeWidget(w)
	app.tab.VerticalSplit(w)
	app.RequestRedraw()
}

func (app *DebuggerApp) EnterInsertMode(args ...any) {
	app.leaveLuaMode()
	app.tab.SetInsertActive(true)
	app.SetMode(platform.ModeInsert)
	app.RequestRedraw()
}

func (app *DebuggerApp) ClearFocus(args ...any) {
	w := app.focusedWidget()
	if c, ok := w.(termui.Clearable); ok {
		c.Clear()
	}
	app.RequestFrame()
}

// OnHelp opens the Viewport user manual in the focused pane (:help).
func (app *DebuggerApp) OnHelp(args ...any) {
	if app.helpWidget == nil || app.tab == nil {
		return
	}
	if app.swapFocusedWidget(app.helpWidget) {
		app.RequestFrame()
	}
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
	app.Debug().SetClearOutput(true)
}

func (app *DebuggerApp) SetClearOutputOff(args ...any) {
	app.Debug().SetClearOutput(false)
}

func (app *DebuggerApp) SetContinueAfterClearOn(args ...any) {
	app.Debug().SetContinueAfterClear(true)
}

func (app *DebuggerApp) SetContinueAfterClearOff(args ...any) {
	app.Debug().SetContinueAfterClear(false)
}

func (app *DebuggerApp) SetEscToCodeOn(args ...any) {
	app.State().SetEscToCode(true)
}

func (app *DebuggerApp) SetEscToCodeOff(args ...any) {
	app.State().SetEscToCode(false)
}

func (app *DebuggerApp) SetBreakMainOn(args ...any) {
	app.Debug().SetBreakMain(true)
	app.maybeBreakMain()
}

func (app *DebuggerApp) SetBreakMainOff(args ...any) {
	app.Debug().SetBreakMain(false)
}

func (app *DebuggerApp) SetGdbListenPrintOn(args ...any) {
	app.Debug().SetGdbListenPrint(true)
}

func (app *DebuggerApp) SetGdbListenPrintOff(args ...any) {
	app.Debug().SetGdbListenPrint(false)
}

func (app *DebuggerApp) SetGdbTargetPrintOn(args ...any) {
	app.Debug().SetGdbTargetPrint(true)
}

func (app *DebuggerApp) SetGdbTargetPrintOff(args ...any) {
	app.Debug().SetGdbTargetPrint(false)
}

// SetInferiorTTYCmd handles:
//
//	:set inferior-tty              — open an external terminal and route stdio there
//	:set inferior-tty internal     — restore the in-app IO pane
//	:set inferior-tty /dev/pts/N   — use an already-open slave (advanced)
func (app *DebuggerApp) SetInferiorTTYCmd(args ...any) {
	path := strings.TrimSpace(joinCmdArgs(args))
	if path == "" {
		pts, err := app.OpenExternalTTY()
		if err != nil {
			if app.ctx.Log != nil {
				app.ctx.Log.Named("set").Error(err.Error())
			}
			return
		}
		path = pts
	}
	if err := app.SetInferiorTTY(path); err != nil && app.ctx.Log != nil {
		app.ctx.Log.Named("set").Error(err.Error())
	}
	app.RequestFrame()
}

// inferiorTTYCompletions is Tab after :set inferior-tty.
func (app *DebuggerApp) inferiorTTYCompletions(prefix string) []string {
	cands := []string{"internal"}
	var out []string
	for _, c := range cands {
		if prefix == "" || strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

func (app *DebuggerApp) SetMarkColor(args ...any) {
	name := joinCmdArgs(args)
	if name == "" {
		return
	}
	c, ok := platform.ParseColorName(name)
	if !ok {
		if app.ctx.Log != nil {
			app.ctx.Log.Named("set").Error("unknown markcolor: " + name)
		}
		return
	}
	app.State().SetMarkColor(c)
	app.RequestFrame()
}

func (app *DebuggerApp) SetMarkDimColor(args ...any) {
	name := joinCmdArgs(args)
	if name == "" {
		return
	}
	c, ok := platform.ParseColorName(name)
	if !ok {
		if app.ctx.Log != nil {
			app.ctx.Log.Named("set").Error("unknown markdimcolor: " + name)
		}
		return
	}
	app.State().SetMarkDimColor(c)
	app.RequestFrame()
}

func (app *DebuggerApp) SetBreakColor(args ...any) {
	name := joinCmdArgs(args)
	if name == "" {
		return
	}
	c, ok := platform.ParseColorName(name)
	if !ok {
		if app.ctx.Log != nil {
			app.ctx.Log.Named("set").Error("unknown breakcolor: " + name)
		}
		return
	}
	app.Debug().SetBreakColor(c)
	app.rebuildCodeBreakGutters()
	app.RequestFrame()
}

func (app *DebuggerApp) SetBreakDisabledColor(args ...any) {
	name := joinCmdArgs(args)
	if name == "" {
		return
	}
	c, ok := platform.ParseColorName(name)
	if !ok {
		if app.ctx.Log != nil {
			app.ctx.Log.Named("set").Error("unknown breakdisabledcolor: " + name)
		}
		return
	}
	app.Debug().SetBreakDisabledColor(c)
	app.rebuildCodeBreakGutters()
	app.RequestFrame()
}

func (app *DebuggerApp) SetPCColor(args ...any) {
	name := joinCmdArgs(args)
	if name == "" {
		return
	}
	c, ok := platform.ParseColorName(name)
	if !ok {
		if app.ctx.Log != nil {
			app.ctx.Log.Named("set").Error("unknown pccolor: " + name)
		}
		return
	}
	app.Debug().SetPCColor(c)
	app.RequestFrame()
}

func (app *DebuggerApp) SetStackBreakColor(args ...any) {
	name := joinCmdArgs(args)
	if name == "" {
		return
	}
	c, ok := platform.ParseColorName(name)
	if !ok {
		if app.ctx.Log != nil {
			app.ctx.Log.Named("set").Error("unknown stackbreakcolor: " + name)
		}
		return
	}
	app.Debug().SetStackBreakColor(c)
	app.RequestFrame()
}

func (app *DebuggerApp) SetCodeSelColor(args ...any) {
	name := joinCmdArgs(args)
	if name == "" {
		return
	}
	c, ok := platform.ParseColorName(name)
	if !ok {
		if app.ctx.Log != nil {
			app.ctx.Log.Named("set").Error("unknown codeselcolor: " + name)
		}
		return
	}
	app.State().SetCodeSelColor(c)
	app.RequestFrame()
}

func (app *DebuggerApp) SetMutedColor(args ...any) {
	name := joinCmdArgs(args)
	if name == "" {
		return
	}
	c, ok := platform.ParseColorName(name)
	if !ok {
		if app.ctx.Log != nil {
			app.ctx.Log.Named("set").Error("unknown mutedcolor: " + name)
		}
		return
	}
	app.State().SetMutedColor(c)
	app.RequestFrame()
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
	argv := anyArgsToStrings(args)
	if len(argv) == 0 {
		return
	}
	w := app.startExecSession(argv)
	if w == nil {
		return
	}
	if app.tab != nil && app.swapFocusedWidget(w) {
		app.EnterInsertMode()
		app.RequestFrame()
	}
}

// SpawnExec starts argv in the background without stealing focus (JLink / gdbserver).
// Registers :b exec so logs remain optional. Used by gdbforge.spawn.
// Returns an error if the process failed to start.
func (app *DebuggerApp) SpawnExec(argv []string) error {
	w := app.startExecSession(argv)
	if w == nil {
		return fmt.Errorf("spawn failed: could not start %v", argv)
	}
	// Winsize normally comes from Draw; background exec never draws — set a default.
	if app.execClient != nil {
		_ = app.execClient.SetSize(24, 80)
	}
	if app.ctx.Log != nil {
		app.ctx.Log.Named("exec").Info("spawned: " + strings.Join(argv, " "))
	}
	app.RequestFrame()
	return nil
}

func anyArgsToStrings(args []any) []string {
	argv := make([]string, 0, len(args))
	for _, a := range args {
		s, ok := a.(string)
		if !ok || s == "" {
			continue
		}
		argv = append(argv, s)
	}
	return argv
}

// startExecSession creates ExecClient + ExecWidget and registers builtin "exec".
// Returns nil on error. Does not change focus.
func (app *DebuggerApp) startExecSession(argv []string) *widgets.ExecWidget {
	if len(argv) == 0 {
		return nil
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
		return nil
	}
	app.execClient = client

	w := widgets.NewExecWidget()
	w.SetClipboard(app.ClipboardIO())
	w.SetSizeFunc(client.SetSize)
	w.SetOnSubmit(func(cmd string) {
		_ = client.Send(cmd)
	})
	w.SetOnInterrupt(func() {
		_ = client.SendRaw("\x03")
	})
	w.SetOnSuspend(func() {
		_ = client.SendRaw("\x1a")
	})
	w.SetOnEOF(func() {
		_ = client.SendRaw("\x04")
	})
	w.SetOnClose(func() {
		client.Close()
	})
	w.SetOnDismiss(func() {
		if app.execClient != nil {
			app.execClient.Close()
			app.execClient = nil
		}
		app.execWidget = nil
		if app.builtins != nil {
			delete(app.builtins, "exec")
		}
		app.JumpBack()
		app.RequestFrame()
	})
	ch, _ := client.Subscribe()
	w.StartExecUIBridge(app.Screen(), ch)
	app.execWidget = w
	app.registerBuiltin("exec", w)
	return w
}
