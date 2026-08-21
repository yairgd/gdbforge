package main

import (
	"context"
	"os"
	"path/filepath"
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

	got := a.lua.completions("remotegdb ", false)
	if !reflect.DeepEqual(got, []string{"help"}) {
		t.Fatalf("after script+space: %v want [help]", got)
	}

	got = a.lua.completions("remotegdb he", false)
	if !reflect.DeepEqual(got, []string{"help"}) {
		t.Fatalf("partial help: %v want [help]", got)
	}

	got = a.lua.completions("remotegdb -", false)
	wantDash := []string{"-h", "--help"}
	if !reflect.DeepEqual(got, wantDash) {
		t.Fatalf("dash help: %v want %v", got, wantDash)
	}

	got = a.lua.completions("re", false)
	wantRE := []string{"remotegdb", "repl"}
	if !reflect.DeepEqual(got, wantRE) {
		t.Fatalf("script prefix: %v want %v", got, wantRE)
	}

	got = a.lua.completions("help", false)
	if len(got) != 1 || got[0] != "help" {
		t.Fatalf(":lua help completion: %v", got)
	}

	got = a.lua.completions("remotegdb", false)
	if len(got) != 1 || got[0] != "remotegdb" {
		t.Fatalf("full script name still completes as script: %v", got)
	}
}

func TestLuaCompletionsFromScript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo_complete.lua")
	const src = `
function main() end
gdbforge.complete_args(function(token, index)
  if index == 1 then
    local all = {"alpha", "beta", "help"}
    if token == "" then return all end
    local out = {}
    for _, v in ipairs(all) do
      if v:sub(1, #token) == token then out[#out+1] = v end
    end
    return out
  end
  return {}
end)
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	a := &DebuggerApp{}
	a.lua.app = a
	a.lua.pending = map[string]luahost.ResolvedScript{
		"demo_complete": {Cmd: "demo_complete", Path: path},
	}

	got := a.lua.completions("demo_complete ", false)
	want := []string{"alpha", "beta", "help"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("complete_args all: %v want %v", got, want)
	}

	got = a.lua.completions("demo_complete a", false)
	if !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("complete_args prefix: %v", got)
	}
}
