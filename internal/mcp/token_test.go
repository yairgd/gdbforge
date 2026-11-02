package mcp

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yairgd/termforge/ptyx"
)

// tokenSession answers every command with reply(token, command), after delay, the way GDB
// echoes the token of the command in its result record.
type tokenSession struct {
	fakeSession
	delay time.Duration
	reply func(tok, cmd string) string
}

var tokRe = regexp.MustCompile(`^(\d+)(.*)$`)

func (f *tokenSession) WithWrite(_ context.Context, fn func(w ptyx.PTYWriter) error) error {
	return fn(tokenWriter{f})
}

type tokenWriter struct{ f *tokenSession }

func (w tokenWriter) Send(cmd string) error {
	m := tokRe.FindStringSubmatch(cmd)
	go func() {
		time.Sleep(w.f.delay)
		w.f.ch <- ptyx.PtyOutputMsg{Data: w.f.reply(m[1], m[2])}
	}()
	return nil
}
func (w tokenWriter) SendRaw(string) error { return nil }

func TestQueryTokenWaitsPastIdleGapAndIgnoresStaleReplies(t *testing.T) {
	sess := &tokenSession{fakeSession: fakeSession{ch: make(chan ptyx.PtyOutputMsg, 8)}, delay: 300 * time.Millisecond}
	sess.reply = func(tok, cmd string) string {
		// a stale reply of an older command first, then ours
		return "1^done,value=\"stale\"\n(gdb)\n" + tok + "^done,value=\"7\"\n(gdb)\n"
	}
	s := NewGdbMcpService(sess, nil)
	out, err := s.QueryToken(context.Background(), "-data-evaluate-expression x", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `^done,value="7"`) {
		t.Fatalf("reply=%q", out)
	}
}

func TestQueryTokenTimeout(t *testing.T) {
	sess := &tokenSession{fakeSession: fakeSession{ch: make(chan ptyx.PtyOutputMsg, 8)}, delay: time.Hour}
	sess.reply = func(string, string) string { return "" }
	s := NewGdbMcpService(sess, nil)
	_, err := s.QueryToken(context.Background(), "info registers", 50*time.Millisecond)
	var ae *APIError
	if !errors.As(err, &ae) || ae.Code != "timeout" {
		t.Fatalf("err=%v, want timeout APIError", err)
	}
}

func TestFormatReply(t *testing.T) {
	raw := "&\"info registers pc\\n\"\n~\"pc             0x401136            0x401136 <main+4>\\n\"\n@\"from target\\n\"\n1005^done\n(gdb)\n"
	got, err := FormatReply(raw, "")
	if err != nil || got != "pc             0x401136            0x401136 <main+4>\nfrom target" {
		t.Fatalf("console=%q err=%v", got, err)
	}
	if got, _ := FormatReply(raw, "mi"); got != raw {
		t.Fatalf("mi must be the raw reply, got %q", got)
	}
	if got, _ := FormatReply("5^done\n", ""); got != "(no output)" {
		t.Fatalf("empty=%q", got)
	}
	if got, _ := FormatReply("1007^done,value=\"42\"\n(gdb)\n", ""); got != `value="42"` {
		t.Fatalf("MI result data lost: %q", got)
	}
	if _, err := FormatReply("5^done\n", "xml"); err == nil {
		t.Fatal("bad format must fail")
	}
}

func TestFormatReplyMapsRunningErrors(t *testing.T) {
	_, err := FormatReply(`9^error,msg="Cannot execute this command while the selected thread is running."`+"\n", "")
	var ae *APIError
	if !errors.As(err, &ae) || ae.Code != "target_running" {
		t.Fatalf("err=%v, want target_running", err)
	}
	_, err = FormatReply(`9^error,msg="No symbol \"zz\" in current context."`+"\n", "")
	if err == nil || errors.As(err, &ae) {
		t.Fatalf("a plain error must stay plain, got %v", err)
	}
}
