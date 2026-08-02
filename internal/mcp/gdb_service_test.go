package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
)

type fakeSession struct {
	writes []string
	ch     chan core.PtyOutputMsg
}

func (f *fakeSession) Send(cmd string) error {
	return f.WithWrite(context.Background(), func(w core.PTYWriter) error {
		return w.Send(cmd)
	})
}
func (f *fakeSession) SendRaw(raw string) error {
	return f.WithWrite(context.Background(), func(w core.PTYWriter) error {
		return w.SendRaw(raw)
	})
}
func (f *fakeSession) Close() {}
func (f *fakeSession) Subscribe() (<-chan core.PtyOutputMsg, func()) {
	return f.ch, func() {}
}
func (f *fakeSession) WithWrite(_ context.Context, fn func(w core.PTYWriter) error) error {
	return fn(fakeWriter{f})
}

type fakeWriter struct{ f *fakeSession }

func (w fakeWriter) Send(cmd string) error {
	w.f.writes = append(w.f.writes, cmd+"\n")
	select {
	case w.f.ch <- core.PtyOutputMsg{Data: ">>> " + cmd + "\n(gdb) "}:
	default:
	}
	return nil
}
func (w fakeWriter) SendRaw(raw string) error {
	w.f.writes = append(w.f.writes, raw)
	return nil
}

func TestGdbCommandCapturesOutput(t *testing.T) {
	ch := make(chan core.PtyOutputMsg, 8)
	sess := &fakeSession{ch: ch}
	svc := NewGdbMcpService(sess, nil)
	svc.captureIdle = 80 * time.Millisecond
	svc.captureMax = 500 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := svc.GdbCommand(ctx, "b main")
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.writes) != 1 || sess.writes[0] != "b main\n" {
		t.Fatalf("writes=%v", sess.writes)
	}
	if !strings.Contains(out, "b main") {
		t.Fatalf("output=%q", out)
	}
}
