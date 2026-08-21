package main

import (
	"context"
	"testing"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
)

type noopSession struct{}

func (noopSession) Send(string) error      { return nil }
func (noopSession) SendRaw(string) error   { return nil }
func (noopSession) Close()                 {}
func (noopSession) Subscribe() (<-chan core.PtyOutputMsg, func()) {
	return nil, func() {}
}
func (noopSession) WithWrite(context.Context, func(core.PTYWriter) error) error {
	return nil
}

type stubDebugInfoHost struct {
	st           *platform.AppState
	debug        *debugstate.State
	noteStackNav int
	bumpCodeNav  int
	suppressDlv  int
	showCode     int
	dlv          bool
}

func (s *stubDebugInfoHost) Backend() backend.Backend { return nil }
func (s *stubDebugInfoHost) Session() core.Session    { return noopSession{} }
func (s *stubDebugInfoHost) State() *platform.AppState {
	if s.st == nil {
		s.st = platform.NewAppState()
	}
	return s.st
}
func (s *stubDebugInfoHost) Debug() *debugstate.State {
	if s.debug == nil {
		s.debug = debugstate.New(s.State())
	}
	return s.debug
}
func (s *stubDebugInfoHost) GdbMcp() *mcp.GdbMcpService        { return nil }
func (s *stubDebugInfoHost) GDBWidget() *widgets.GDBWidget     { return widgets.NewGDBWidget() }
func (s *stubDebugInfoHost) Screen() tcell.Screen              { return nil }
func (s *stubDebugInfoHost) RequestFrame()                     {}
func (s *stubDebugInfoHost) BumpCodeNav()                      { s.bumpCodeNav++ }
func (s *stubDebugInfoHost) NoteStackNavGDB()                  { s.noteStackNav++ }
func (s *stubDebugInfoHost) SuppressDlvStopUI()                { s.suppressDlv++ }
func (s *stubDebugInfoHost) isDLV() bool                       { return s.dlv }
func (s *stubDebugInfoHost) showFrameSource(models.StackFrame) {}
func (s *stubDebugInfoHost) ShowCodeAt(string, int) *widgets.CodeWidget {
	s.showCode++
	return nil
}
func (s *stubDebugInfoHost) LogError(string, string) {}

func TestActivateThreadArmsFrameSyncNotSyncRefresh(t *testing.T) {
	host := &stubDebugInfoHost{}
	c := &debugInfoCtl{
		host:  host,
		stack: &models.CallStack{},
	}
	c.stack.Set([]models.StackFrame{
		{Level: 0, Func: "stale", File: "/stale.c", Line: 1},
	})

	c.activateThread(models.ThreadInfo{
		ID: "2", State: "stopped", File: "/tmp/new.c", Line: 42, Func: "worker",
	})

	if host.noteStackNav != 1 {
		t.Fatalf("NoteStackNavGDB=%d want 1 (defer stack refresh until prompt)", host.noteStackNav)
	}
	if host.bumpCodeNav != 1 {
		t.Fatalf("BumpCodeNav=%d want 1", host.bumpCodeNav)
	}
	if host.showCode != 1 {
		t.Fatalf("ShowCodeAt=%d want 1 (optimistic thread row)", host.showCode)
	}
	if frames := c.stack.Items(); len(frames) != 1 || frames[0].Func != "stale" {
		t.Fatalf("stack model changed synchronously: %+v", frames)
	}
}

func TestActivateThreadDLVSuppressesStopUI(t *testing.T) {
	host := &stubDebugInfoHost{dlv: true}
	c := &debugInfoCtl{host: host}
	c.activateThread(models.ThreadInfo{ID: "3", State: "running"})
	if host.suppressDlv != 1 {
		t.Fatalf("SuppressDlvStopUI=%d want 1", host.suppressDlv)
	}
	if host.noteStackNav != 0 {
		t.Fatalf("NoteStackNavGDB=%d want 0 for Delve", host.noteStackNav)
	}
}

func TestActivateThreadKgdbUsesLegacyPath(t *testing.T) {
	host := &stubDebugInfoHost{}
	host.Debug().SetKgdbMode(true)
	c := &debugInfoCtl{
		host:  host,
		stack: &models.CallStack{},
	}
	c.stack.Set([]models.StackFrame{
		{Level: 0, Func: "keep", File: "/keep.c", Line: 1},
	})

	c.activateThread(models.ThreadInfo{
		ID: "2", State: "stopped", File: "/tmp/work.c", Line: 7, Func: "worker",
	})

	if host.noteStackNav != 0 {
		t.Fatalf("NoteStackNavGDB=%d want 0 (kgdb unchanged)", host.noteStackNav)
	}
	if host.bumpCodeNav != 0 {
		t.Fatalf("BumpCodeNav=%d want 0 (kgdb unchanged)", host.bumpCodeNav)
	}
	// nil GdbMcp → refreshThreadsAndStack is a no-op; stack must not be replaced
	// by the single-frame shortcut used for non-kgdb paths.
	if frames := c.stack.Items(); len(frames) != 1 || frames[0].Func != "keep" {
		t.Fatalf("kgdb stack model unexpectedly changed: %+v", frames)
	}
}
