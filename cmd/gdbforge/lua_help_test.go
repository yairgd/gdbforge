package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/yairgd/gdbforge/internal/luahost"
)

func TestCancelLuaJob(t *testing.T) {
	a := &DebuggerApp{}
	if a.cancelLuaJob() {
		t.Fatal("no job")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.luaJobMu.Lock()
	a.luaJobCancel = cancel
	a.luaJobBusy.Store(true)
	a.luaJobMu.Unlock()
	if !a.cancelLuaJob() {
		t.Fatal("expected cancel")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("context not cancelled")
	}
}

func TestCallOnUIPassthrough(t *testing.T) {
	a := &DebuggerApp{}
	called := false
	a.callOnUI(func() { called = true })
	if !called {
		t.Fatal("callOnUI must run inline when not on worker")
	}
}

func TestIsLuaHelpRequest(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{}, false},
		{[]string{"help"}, true},
		{[]string{"-h"}, true},
		{[]string{"--help"}, true},
		{[]string{" help "}, true},
		{[]string{"./help"}, false},
		{[]string{"help", "extra"}, false},
		{[]string{"hello"}, false},
	}
	for _, tc := range cases {
		if got := isLuaHelpRequest(tc.args); got != tc.want {
			t.Fatalf("args=%v got %v want %v", tc.args, got, tc.want)
		}
	}
}

func TestLuaCompletionsHelp(t *testing.T) {
	a := &DebuggerApp{
		luaPending: map[string]luahost.ResolvedScript{
			"remotegdb": {Cmd: "remotegdb"},
			"snake":     {Cmd: "snake"},
		},
	}

	got := a.luaCompletions("remotegdb ")
	if !reflect.DeepEqual(got, []string{"help"}) {
		t.Fatalf("after script+space: %v want [help]", got)
	}

	got = a.luaCompletions("remotegdb he")
	if !reflect.DeepEqual(got, []string{"help"}) {
		t.Fatalf("partial help: %v want [help]", got)
	}

	got = a.luaCompletions("remotegdb -")
	wantDash := []string{"-h", "--help"}
	if !reflect.DeepEqual(got, wantDash) {
		t.Fatalf("dash help: %v want %v", got, wantDash)
	}

	got = a.luaCompletions("re")
	if len(got) != 1 || got[0] != "remotegdb" {
		t.Fatalf("script prefix: %v", got)
	}

	got = a.luaCompletions("remotegdb")
	if len(got) != 1 || got[0] != "remotegdb" {
		t.Fatalf("full script name still completes as script: %v", got)
	}
}
