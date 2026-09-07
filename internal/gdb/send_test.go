package gdb

import (
	"context"
	"fmt"
	"testing"

	"github.com/yairgd/gdbforge/internal/core"
)

func TestIsContinueCmd(t *testing.T) {
	yes := []string{"c", "continue", "  continue  ", "c\n"}
	no := []string{"", "n", "next", "s", "step", "run", "target remote /dev/pts/1"}
	for _, cmd := range yes {
		if !IsContinueCmd(cmd) {
			t.Fatalf("IsContinueCmd(%q) = false, want true", cmd)
		}
	}
	for _, cmd := range no {
		if IsContinueCmd(cmd) {
			t.Fatalf("IsContinueCmd(%q) = true, want false", cmd)
		}
	}
}

type sendSess struct {
	writes   []string
	failCmds map[string]bool
}

func (s *sendSess) Send(string) error    { return nil }
func (s *sendSess) SendRaw(string) error { return nil }
func (s *sendSess) Close()               {}
func (s *sendSess) Subscribe() (<-chan core.PtyOutputMsg, func()) {
	return nil, func() {}
}
func (s *sendSess) WithWrite(_ context.Context, fn func(core.PTYWriter) error) error {
	return fn(sendWriter{s})
}

type sendWriter struct{ s *sendSess }

func (w sendWriter) Send(cmd string) error {
	if w.s.failCmds[cmd] {
		return fmt.Errorf("write %q failed", cmd)
	}
	w.s.writes = append(w.s.writes, cmd)
	return nil
}
func (w sendWriter) SendRaw(raw string) error { w.s.writes = append(w.s.writes, raw); return nil }

type sendCtl struct {
	running    bool
	afterClear bool
	suppress   int
}

func (c *sendCtl) InferiorRunning() bool      { return c.running }
func (c *sendCtl) ContinueAfterClear() bool   { return c.afterClear }
func (c *sendCtl) NoteTransientStopSuppress() { c.suppress++ }

func wantWrites(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("writes=%q want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("writes=%q want %q", got, want)
		}
	}
}

func TestSendCmdBreakWhileRunningInterruptsOnCommandChannel(t *testing.T) {
	sess := &sendSess{}
	ctl := &sendCtl{running: true}

	SendCmd(sess, nil, ctl, "break hello.c:11", SendOpts{InterruptCmd: MIExecInterrupt})

	wantWrites(t, sess.writes, []string{MIExecInterrupt, "break hello.c:11", "continue"})
	if ctl.suppress != 1 {
		t.Fatalf("suppress=%d want 1", ctl.suppress)
	}
}

func TestSendCmdWhileStoppedSendsPlainCommand(t *testing.T) {
	sess := &sendSess{}
	ctl := &sendCtl{}

	SendCmd(sess, nil, ctl, "break hello.c:11", SendOpts{InterruptCmd: MIExecInterrupt})

	wantWrites(t, sess.writes, []string{"break hello.c:11"})
}

func TestSendCmdFallsBackToInlineCtrlC(t *testing.T) {
	sess := &sendSess{}
	ctl := &sendCtl{running: true}

	SendCmd(sess, nil, ctl, "break hello.c:11", SendOpts{})

	wantWrites(t, sess.writes, []string{"\x03", "break hello.c:11", "continue"})
}
