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
	app.Tab().FocusLeft()
	app.rememberCodeLeafFromFocus()
}

func (app *DebuggerApp) OnFocusRight(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send right command")

	app.Tab().FocusRight()
	app.rememberCodeLeafFromFocus()
}

func (app *DebuggerApp) OnFocusUp(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send up command")

	app.Tab().FocusUp()
	app.rememberCodeLeafFromFocus()
}

func (app *DebuggerApp) OnFocusDown(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("send down command")

	app.Tab().FocusDown()
	app.rememberCodeLeafFromFocus()
}

func (app *DebuggerApp) BreakFile(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("break file")
}

func (app *DebuggerApp) CmdDeleteBreakpoint(args ...any) {
	log := app.ctx.Log.Named("MainApp")
	log.Info("delete breakpoint")
}

// :gdb run — start/restart inferior (GDB: run; Delve: restart).
func (app *DebuggerApp) GdbRun(args ...any) {
	app.sendGdbExec("run")
	app.RequestFrame()
}

// :gdb start — GDB start (break at main + run); Delve maps to restart.
func (app *DebuggerApp) GdbStart(args ...any) {
	app.sendGdbExec("start")
	app.RequestFrame()
}

func (app *DebuggerApp) GdbContinue(args ...any) {
	app.sendGdbExec("continue")
	app.RequestFrame()
}

func (app *DebuggerApp) GdbNext(args ...any) {
	app.sendGdbExec("next")
	app.RequestFrame()
}

func (app *DebuggerApp) GdbStep(args ...any) {
	app.sendGdbExec("step")
	app.RequestFrame()
}

func (app *DebuggerApp) GdbFinish(args ...any) {
	app.sendGdbExec("finish")
	app.RequestFrame()
}

func (app *DebuggerApp) GdbNexti(args ...any) {
	app.sendGdbExec("nexti")
	app.RequestFrame()
}

func (app *DebuggerApp) GdbStepi(args ...any) {
	app.sendGdbExec("stepi")
	app.RequestFrame()
}

func (app *DebuggerApp) GdbInterrupt(args ...any) {
	app.console.onGdbConsoleInterrupt()
	app.RequestFrame()
}

func (app *DebuggerApp) ShowRegisters(args ...any) {
	cmd := "info registers"
	if app.backend != nil {
		cmd = app.backend.InfoRegistersCmd()
	}
	app.sendGdbExec(cmd)
	app.RequestFrame()
}

func (app *DebuggerApp) ShowThreads(args ...any) {
	cmd := "info threads"
	if app.backend != nil {
		cmd = app.backend.InfoThreadsCmd()
	}
	app.sendGdbExec(cmd)
	app.RequestFrame()
}

func (app *DebuggerApp) SplitHorizontal(args ...any) {
	if isAsmSplitArg(args...) {
		app.SplitAsmBelow()
		return
	}
	w := app.Widgets()[0].Widget()
	tab, ok := w.(*termui.TabWidget)
	if !ok {
		return
	}

	l := termui.NewLoggerWidget(app.ctx)
	l.SetClipboard(app.ClipboardIO())
	tab.HorizontalSplit(l)
	app.RequestRedraw()
}

func (app *DebuggerApp) SplitVertical(args ...any) {
	if isAsmSplitArg(args...) {
		app.SplitAsmRight()
		return
	}
	w := widgets.NewCodeWidget()
	w.PaneName = "[No Name]"
	w.SetClipboard(app.ClipboardIO())
	app.bufs.wire(w)
	app.Tab().VerticalSplit(w)
	app.RequestRedraw()
}

// splitAsmCompletions is Tab after :vs / :sp (:vs asm).
func (a *DebuggerApp) splitAsmCompletions(prefix string, _ bool) []string {
	var out []string
	for _, name := range []string{"asm", "assembly"} {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	return out
}

func isAsmSplitArg(args ...any) bool {
	name := strings.ToLower(strings.TrimSpace(joinCmdArgs(args)))
	return name == "asm" || name == "assembly"
}

func (app *DebuggerApp) EnterInsertMode(args ...any) {
	app.lua.leaveMode()
	app.Tab().SetInsertActive(true)
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

// OnlyFocus closes every pane except the focused one (Vim Ctrl-W o / :only).
func (app *DebuggerApp) OnlyFocus(args ...any) {
	if app.Tab() == nil {
		return
	}
	if !app.Tab().OnlyFocus() {
		return
	}
	app.rememberCodeLeafFromFocus()
	app.RequestRedraw()
}

// OnHelp opens the Viewport user manual in the focused pane (:help).
func (app *DebuggerApp) OnHelp(args ...any) {
	if app.helpWidget == nil || app.Tab() == nil {
		return
	}
	if app.swapFocusedWidget(app.helpWidget) {
		app.RequestFrame()
	}
}

func (app *DebuggerApp) Quit(args ...any) {
	// :q / :quit — confirm when inferior alive (same as Ctrl-D).
	// :q! / :quit! — force exit; teardown runs via defer app.Close() after Run().
	if cmdArgsHasBang(args) {
		app.Exit()
		return
	}
	app.console.onGdbConsoleEOF()
	app.RequestFrame()
}

func cmdArgsHasBang(args []any) bool {
	for _, a := range args {
		if s, ok := a.(string); ok && s == "!" {
			return true
		}
	}
	return false
}

// ClosePane removes the focused split (:close). Does not exit the app when
// only one pane remains (unlike the old vim-style :quit).
func (app *DebuggerApp) ClosePane(args ...any) {
	if app.Tab() == nil {
		return
	}
	if app.Tab().DeleteFocus() {
		// Last pane — nothing to close; stay in the session.
		return
	}
	app.RequestRedraw()
}

func (app *DebuggerApp) SetEqualAlwaysOn(args ...any) {
	app.State().SetEqualAlways(true)
	if app.Tab() != nil {
		app.Tab().SetEqualAlways(true)
		if tree := app.Tab().ActiveTree(); tree != nil {
			tree.Rebalance()
		}
	}
	app.RequestFrame()
}

func (app *DebuggerApp) SetEqualAlwaysOff(args ...any) {
	app.State().SetEqualAlways(false)
	if app.Tab() != nil {
		app.Tab().SetEqualAlways(false)
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
	app.breaks.maybeBreakMain()
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

// SetLogCmd handles:
//
//	:set log              — append structured logs to gdbforge.log
//	:set log <file>       — append structured logs to <file>
func (app *DebuggerApp) SetLogCmd(args ...any) {
	path := strings.TrimSpace(joinCmdArgs(args))
	if err := app.enableFileLog(path); err != nil && app.ctx.Log != nil {
		app.ctx.Log.Named("set").Error(err.Error())
	}
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
func (app *DebuggerApp) inferiorTTYCompletions(prefix string, _ bool) []string {
	cands := []string{"internal"}
	var out []string
	for _, c := range cands {
		if prefix == "" || strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

// colorCompletions is Tab after :set breakcolor / markcolor / ….
func (app *DebuggerApp) colorCompletions(prefix string, _ bool) []string {
	return platform.CompleteColorName(prefix)
}

func (app *DebuggerApp) setNamedColor(args []any, what string, apply func(tcell.Color), rebuildGutters bool) {
	name := joinCmdArgs(args)
	if name == "" {
		return
	}
	c, ok := platform.ParseColorName(name)
	if !ok {
		if app.ctx.Log != nil {
			app.ctx.Log.Named("set").Error("unknown " + what + ": " + name)
		}
		return
	}
	apply(c)
	if rebuildGutters {
		app.breaks.rebuildGutters()
	}
	app.RequestFrame()
}

func (app *DebuggerApp) SetMarkColor(args ...any) {
	app.setNamedColor(args, "markcolor", app.State().SetMarkColor, false)
}

func (app *DebuggerApp) SetMarkDimColor(args ...any) {
	app.setNamedColor(args, "markdimcolor", app.State().SetMarkDimColor, false)
}

func (app *DebuggerApp) SetBreakColor(args ...any) {
	app.setNamedColor(args, "breakcolor", app.Debug().SetBreakColor, true)
}

func (app *DebuggerApp) SetBreakDisabledColor(args ...any) {
	app.setNamedColor(args, "breakdisabledcolor", app.Debug().SetBreakDisabledColor, true)
}

func (app *DebuggerApp) SetBreakCondColor(args ...any) {
	app.setNamedColor(args, "breakcondcolor", app.Debug().SetBreakCondColor, true)
}

func (app *DebuggerApp) SetPCColor(args ...any) {
	app.setNamedColor(args, "pccolor", app.Debug().SetPCColor, false)
}

func (app *DebuggerApp) SetStackBreakColor(args ...any) {
	app.setNamedColor(args, "stackbreakcolor", app.Debug().SetStackBreakColor, false)
}

func (app *DebuggerApp) SetCodeSelColor(args ...any) {
	app.setNamedColor(args, "codeselcolor", app.State().SetCodeSelColor, false)
}

func (app *DebuggerApp) SetMutedColor(args ...any) {
	app.setNamedColor(args, "mutedcolor", app.State().SetMutedColor, false)
}

func (app *DebuggerApp) SetSearchColor(args ...any) {
	app.setNamedColor(args, "searchcolor", app.State().SetSearchColor, false)
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
	if app.Tab() != nil && app.swapFocusedWidget(w) {
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
	app.children.Track(client.Pid(), true)

	w := widgets.NewExecWidget()
	w.Ctx = app.ctx
	w.SetClipboard(app.ClipboardIO())
	w.WireExec(client.TTY, app.RequestFrame)
	ch, _ := client.Subscribe()
	go func() {
		for range ch {
		}
		w.NotifyExecEnded(app.Screen())
	}()
	app.execWidget = w
	app.registerBuiltin("exec", w)
	return w
}
