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

func TestSendCmdFrameWhileRunningDoesNotContinue(t *testing.T) {
	sent := make(chan string, 8)
	sess := &fakeSess{sent: sent}
	st := platform.NewAppState()
	st.SetInferiorRunning(true)

	SendCmd(sess, st, "frame 2")

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
	st := platform.NewAppState()
	st.SetInferiorRunning(true)

	SendCmd(sess, st, "thread 1")

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
	st := platform.NewAppState()
	st.SetInferiorRunning(true)

	SendCmd(sess, st, "break hello.c:10")

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
