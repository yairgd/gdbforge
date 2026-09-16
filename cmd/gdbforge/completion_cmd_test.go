package main

import (
	"testing"

	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/termforge"
	"github.com/yairgd/termforge/commands"
	"github.com/yairgd/termforge/platform"
	"github.com/yairgd/termforge/ptyx"
)

type completionTestHost struct {
	mode platform.Mode
	cmd  *termforge.CmdWidget
}

func (h *completionTestHost) GDBWidget() *widgets.GDBWidget                 { return nil }
func (h *completionTestHost) LuaConsoleWidget() *widgets.LuaConsoleWidget   { return nil }
func (h *completionTestHost) LuaGdbforgeComplete(string) (string, []string) { return "", nil }
func (h *completionTestHost) CmdWidget() *termforge.CmdWidget               { return h.cmd }
func (h *completionTestHost) Backend() backend.Backend                      { return nil }
func (h *completionTestHost) Session() ptyx.Session                         { return nil }
func (h *completionTestHost) State() *platform.AppState                     { return nil }
func (h *completionTestHost) Mode() platform.Mode                           { return h.mode }
func (h *completionTestHost) SetMode(m platform.Mode)                       { h.mode = m }
func (h *completionTestHost) IsConfirming() bool                            { return false }
func (h *completionTestHost) PublishCompletion(termforge.CompletionMsg)     {}
func (h *completionTestHost) RequestFrame()                                 {}

func TestMaybeEnterCommandCompletionMode(t *testing.T) {
	cmd := termforge.NewCmdWidget(commands.NewCommandRegistry())
	cmd.Activate()
	host := &completionTestHost{mode: platform.ModeCommand, cmd: cmd}
	c := &completionCtl{host: host, menu: &termforge.CompletionMenu{}}

	c.maybeEnterCommandCompletionMode(1)
	if host.mode != platform.ModeCommand {
		t.Fatalf("single match: mode = %v, want command", host.mode)
	}

	c.maybeEnterCommandCompletionMode(3)
	if host.mode != platform.ModeCompletion {
		t.Fatalf("multi match: mode = %v, want completion", host.mode)
	}
}

func TestOnMsgEntersCommandCompletionMode(t *testing.T) {
	cmd := termforge.NewCmdWidget(commands.NewCommandRegistry())
	cmd.Activate()
	host := &completionTestHost{mode: platform.ModeCommand, cmd: cmd}
	c := &completionCtl{host: host, menu: &termforge.CompletionMenu{}}

	c.onMsg(termforge.CompletionMsg{Names: []string{"gdb", "io", "lua"}})
	if host.mode != platform.ModeCompletion {
		t.Fatalf("onMsg: mode = %v, want completion", host.mode)
	}
	if !c.active() {
		t.Fatal("onMsg: expected active wildmenu")
	}
}
