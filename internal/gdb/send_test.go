package gdb

import (
	"context"
	"testing"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/platform"
)

type fakeSess struct {
	sent chan string
}

func (f *fakeSess) Send(cmd string) error {
	f.sent <- cmd
	return nil
}
func (f *fakeSess) SendRaw(raw string) error {
	f.sent <- raw
	return nil
}
func (f *fakeSess) Close() {}
func (f *fakeSess) Subscribe() (<-chan core.PtyOutputMsg, func()) {
	ch := make(chan core.PtyOutputMsg)
	return ch, func() { close(ch) }
}
func (f *fakeSess) WithWrite(_ context.Context, fn func(core.PTYWriter) error) error {
	return fn(f)
}

type stubCtl struct {
	running, contAfterClear bool
	suppressNotes           int
}

func (s *stubCtl) InferiorRunning() bool      { return s.running }
func (s *stubCtl) ContinueAfterClear() bool   { return s.contAfterClear }
func (s *stubCtl) NoteTransientStopSuppress() { s.suppressNotes++ }

func TestSendCmdFrameWhileRunningDoesNotContinue(t *testing.T) {
	sent := make(chan string, 8)
	sess := &fakeSess{sent: sent}
	app := platform.NewAppState()
	ctl := &stubCtl{running: true}

	SendCmd(sess, app, ctl, "frame 2")

	if got := <-sent; got != "\x03" {
		t.Fatalf("interrupt=%q", got)
	}
	if got := <-sent; got != "frame 2" {
		t.Fatalf("cmd=%q", got)
	}
	select {
	case got := <-sent:
		t.Fatalf("unexpected send after frame: %q (no auto-continue)", got)
	default:
	}
}

func TestSendCmdThreadWhileRunningDoesNotContinue(t *testing.T) {
	sent := make(chan string, 8)
	sess := &fakeSess{sent: sent}
	app := platform.NewAppState()
	ctl := &stubCtl{running: true}

	SendCmd(sess, app, ctl, "thread 1")

	if got := <-sent; got != "\x03" {
		t.Fatalf("interrupt=%q", got)
	}
	if got := <-sent; got != "thread 1" {
		t.Fatalf("cmd=%q", got)
	}
	select {
	case got := <-sent:
		t.Fatalf("unexpected send after thread: %q", got)
	default:
	}
}

func TestSendCmdBreakWhileRunningStillContinues(t *testing.T) {
	sent := make(chan string, 8)
	sess := &fakeSess{sent: sent}
	app := platform.NewAppState()
	ctl := &stubCtl{running: true}

	SendCmd(sess, app, ctl, "break hello.c:10")

	if ctl.suppressNotes != 1 {
		t.Fatalf("suppress notes=%d want 1 (armed before Ctrl-C)", ctl.suppressNotes)
	}
	if got := <-sent; got != "\x03" {
		t.Fatalf("interrupt=%q", got)
	}
	if got := <-sent; got != "break hello.c:10" {
		t.Fatalf("cmd=%q", got)
	}
	if got := <-sent; got != "continue" {
		t.Fatalf("continue=%q", got)
	}
}

func TestSendCmdClearWhileRunningNoContinueNoSuppress(t *testing.T) {
	sent := make(chan string, 8)
	sess := &fakeSess{sent: sent}
	app := platform.NewAppState()
	ctl := &stubCtl{running: true, contAfterClear: false}

	SendCmd(sess, app, ctl, "clear hello.c:10")

	if ctl.suppressNotes != 0 {
		t.Fatalf("suppress notes=%d want 0", ctl.suppressNotes)
	}
	if got := <-sent; got != "\x03" {
		t.Fatalf("interrupt=%q", got)
	}
	if got := <-sent; got != "clear hello.c:10" {
		t.Fatalf("cmd=%q", got)
	}
	select {
	case got := <-sent:
		t.Fatalf("unexpected send: %q", got)
	default:
	}
}

func TestSendCmdClearWhileRunningContinuesWhenEnabled(t *testing.T) {
	sent := make(chan string, 8)
	sess := &fakeSess{sent: sent}
	app := platform.NewAppState()
	ctl := &stubCtl{running: true, contAfterClear: true}

	SendCmd(sess, app, ctl, "clear hello.c:10")

	if ctl.suppressNotes != 1 {
		t.Fatalf("suppress notes=%d want 1", ctl.suppressNotes)
	}
	if got := <-sent; got != "\x03" {
		t.Fatalf("interrupt=%q", got)
	}
	if got := <-sent; got != "clear hello.c:10" {
		t.Fatalf("cmd=%q", got)
	}
	if got := <-sent; got != "continue" {
		t.Fatalf("continue=%q", got)
	}
}

func TestSendCmdBreakWhileStoppedNoSuppress(t *testing.T) {
	sent := make(chan string, 8)
	sess := &fakeSess{sent: sent}
	ctl := &stubCtl{running: false}

	SendCmd(sess, nil, ctl, "break hello.c:10")

	if got := <-sent; got != "break hello.c:10" {
		t.Fatalf("cmd=%q", got)
	}
	if ctl.suppressNotes != 0 {
		t.Fatalf("suppress notes=%d want 0", ctl.suppressNotes)
	}
}

func TestIsBreakInsertCmd(t *testing.T) {
	cases := map[string]bool{
		"break main":         true,
		"break hello.c:2":    true,
		"tbreak foo":         true,
		"-break-insert main": true,
		"clear hello.c:2":    false,
		"-break-delete 1":    false,
		"frame 3":            false,
		"thread 1":           false,
		"continue":           false,
	}
	for cmd, want := range cases {
		if got := IsBreakInsertCmd(cmd); got != want {
			t.Fatalf("%q: got %v want %v", cmd, got, want)
		}
	}
}

func TestIsStackNavCmd(t *testing.T) {
	if !IsStackNavCmd("frame 1") || !IsStackNavCmd("f 0") || !IsStackNavCmd("up") {
		t.Fatal("expected stack nav")
	}
	if IsStackNavCmd("break main") || IsStackNavCmd("") {
		t.Fatal("not stack nav")
	}
}

func TestStopNeedsUIRefresh(t *testing.T) {
	if StopNeedsUIRefresh(&MiStopMsg{Reason: "exited"}) {
		t.Fatal("exit should not refresh UI")
	}
	if !StopNeedsUIRefresh(&MiStopMsg{Reason: "breakpoint-hit"}) {
		t.Fatal("breakpoint should refresh")
	}
}
