package backend

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-delve/delve/service/api"

	"github.com/yairgd/gdbforge/internal/dlv"
)

// An rpc2 resume prints no banner, so noteStop must produce the text the console parser reads.
func TestNoteStopText(t *testing.T) {
	var got string
	b := &DLVBackend{OnStopText: func(s string) { got = s }}

	stopped := &api.DebuggerState{CurrentThread: &api.Thread{
		File: "/src/hello.go", Line: 23, PC: 0x4b639b, Function: &api.Function{Name_: "main.main"},
	}}
	b.noteStop(stopped, nil)
	if !strings.HasPrefix(got, "> main.main() /src/hello.go:23 (PC: 0x4b639b)\n") || !strings.HasSuffix(got, dlv.PromptToken+"\n") {
		t.Fatalf("stop text %q", got)
	}
	// and the real parser must accept it as a stop at that line
	in := dlv.NewInputState()
	u := in.PushRaw(got)
	if u.Stopped == nil || u.Stopped.Line != 23 || u.Stopped.Func != "main.main" || !u.PromptReady {
		t.Fatalf("parsed %+v", u)
	}

	// Delve sets Err as well when the process exits; that is an exit, not a failure.
	got = ""
	b.noteStop(&api.DebuggerState{Exited: true, ExitStatus: 3}, errors.New("Process 1 has exited with status 3"))
	u = dlv.NewInputState().PushRaw(got)
	if !u.InferiorExited {
		t.Fatalf("exit text %q parsed %+v", got, u)
	}

	got = ""
	b.noteStop(nil, errors.New("boom"))
	if !strings.HasPrefix(got, "Command failed: boom") {
		t.Fatalf("error text %q", got)
	}
	got = ""
	b.noteStop(&api.DebuggerState{}, nil) // no thread: nothing to report
	if got != "" {
		t.Fatalf("unexpected %q", got)
	}
}
