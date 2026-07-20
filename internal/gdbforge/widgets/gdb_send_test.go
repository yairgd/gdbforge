package widgets

import (
	"testing"

	"github.com/yairgd/gdbforge/internal/platform"
)

func TestSendGdbCmdFrameWhileRunningDoesNotContinue(t *testing.T) {
	sent := make(chan string, 8)
	sess := &fakeSess{sent: sent}
	st := platform.NewAppState()
	st.SetInferiorRunning(true)

	SendGdbCmd(sess, st, "frame 2")

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

func TestSendGdbCmdThreadWhileRunningDoesNotContinue(t *testing.T) {
	sent := make(chan string, 8)
	sess := &fakeSess{sent: sent}
	st := platform.NewAppState()
	st.SetInferiorRunning(true)

	SendGdbCmd(sess, st, "thread 1")

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

func TestSendGdbCmdBreakWhileRunningStillContinues(t *testing.T) {
	sent := make(chan string, 8)
	sess := &fakeSess{sent: sent}
	st := platform.NewAppState()
	st.SetInferiorRunning(true)

	SendGdbCmd(sess, st, "break hello.c:10")

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
		if got := isBreakInsertCmd(cmd); got != want {
			t.Fatalf("%q: got %v want %v", cmd, got, want)
		}
	}
}
