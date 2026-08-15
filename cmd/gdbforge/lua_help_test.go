package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/yairgd/gdbforge/internal/luahost"
)

func TestCancelLuaJob(t *testing.T) {
	a := &DebuggerApp{}
	a.lua.app = a
	if a.lua.cancelJob() {
		t.Fatal("no job")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.lua.jobMu.Lock()
	a.lua.jobCancel = cancel
	a.lua.jobBusy.Store(true)
	a.lua.jobMu.Unlock()
	if !a.lua.cancelJob() {
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
	a.lua.app = a
	called := false
	a.lua.callOnUI(func() { called = true })
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
	a := &DebuggerApp{}
	a.lua.app = a
	a.lua.pending = map[string]luahost.ResolvedScript{
		"remotegdb": {Cmd: "remotegdb"},
		"snake":     {Cmd: "snake"},
	}

	got := a.lua.completions("remotegdb ")
	if !reflect.DeepEqual(got, []string{"help"}) {
		t.Fatalf("after script+space: %v want [help]", got)
	}

	got = a.lua.completions("remotegdb he")
	if !reflect.DeepEqual(got, []string{"help"}) {
		t.Fatalf("partial help: %v want [help]", got)
	}

	got = a.lua.completions("remotegdb -")
	wantDash := []string{"-h", "--help"}
	if !reflect.DeepEqual(got, wantDash) {
		t.Fatalf("dash help: %v want %v", got, wantDash)
	}

	got = a.lua.completions("re")
	wantRE := []string{"remotegdb", "repl"}
	if !reflect.DeepEqual(got, wantRE) {
		t.Fatalf("script prefix: %v want %v", got, wantRE)
	}

	got = a.lua.completions("help")
	if len(got) != 1 || got[0] != "help" {
		t.Fatalf(":lua help completion: %v", got)
	}

	got = a.lua.completions("remotegdb")
	if len(got) != 1 || got[0] != "remotegdb" {
		t.Fatalf("full script name still completes as script: %v", got)
	}
}
