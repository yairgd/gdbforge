package backend

import (
	"testing"

	"github.com/yairgd/gdbforge/internal/gdbforge/debugger"
	"github.com/yairgd/gdbforge/internal/pydbg"
)

func TestPYMapExec(t *testing.T) {
	b := &PYBackend{}
	for _, tc := range []struct {
		in, send string
		runs     bool
	}{
		{"continue", "continue", true},
		{"n", "n", true},
		{" finish ", "finish", true},
		{"run", "restart", true},
		{"start", "restart", true},
		{"bt", "bt", false},
	} {
		send, runs := b.MapExec(tc.in)
		if send != tc.send || runs != tc.runs {
			t.Errorf("MapExec(%q) = %q,%v want %q,%v", tc.in, send, runs, tc.send, tc.runs)
		}
	}
}

func TestPYPromptReady(t *testing.T) {
	for in, want := range map[string]bool{
		"(py) ":                   true,
		"> f() a.py:3\r\n(py) ":   true,
		"\x1b[1m(py)\x1b[0m \r\n": true,
		"(py)":                    true,
		"(py) x = 1":              false,
		"Process has exited":      false,
		"":                        false,
	} {
		if got := promptReady(in); got != want {
			t.Errorf("promptReady(%q) = %v want %v", in, got, want)
		}
	}
}

func TestPYPushConsoleOutput(t *testing.T) {
	b := &PYBackend{}
	if u := b.PushConsoleOutput("(py) "); u.Stopped != nil || !u.PromptReady || u.State != debugger.StateDone {
		t.Fatalf("empty queue: %+v", u)
	}

	b.enqueue(pydbg.Event{Kind: pydbg.Running})
	b.enqueue(pydbg.Event{Kind: pydbg.Stopped, Reason: "breakpoint", ThreadID: "1", File: "a.py", Line: 42, Func: "f"})
	b.enqueue(pydbg.Event{Kind: pydbg.BreaksChanged})
	b.enqueue(pydbg.Event{Kind: pydbg.FrameSelected, Level: 2, File: "b.py", Line: 7, Func: "g"})
	u := b.PushConsoleOutput("> f() a.py:42\n(py) ")
	if u.Stopped == nil || u.Stopped.Reason != "breakpoint" || u.Stopped.File != "a.py" || u.Stopped.Line != 42 ||
		u.Stopped.Func != "f" || u.Stopped.ThreadID != "1" || !u.Stopped.NeedsUIRefresh() {
		t.Errorf("stop: %+v", u.Stopped)
	}
	if u.State == debugger.StateRunning || u.InferiorExited || !u.PromptReady || !u.BreakpointsChanged {
		t.Errorf("flags: %+v", u)
	}
	if f := u.FrameSelected; f == nil || f.Level != 2 || f.File != "b.py" || f.Line != 7 || f.Func != "g" {
		t.Errorf("frame: %+v", f)
	}
	// Drained: nothing fires twice.
	if u := b.PushConsoleOutput("(py) "); u.Stopped != nil || u.BreakpointsChanged || u.FrameSelected != nil {
		t.Errorf("second push: %+v", u)
	}

	// Event after its banner is returned by the next push, not lost.
	if u := b.PushConsoleOutput("> g() a.py:9\n"); u.Stopped != nil || u.PromptReady {
		t.Errorf("banner first: %+v", u)
	}
	b.enqueue(pydbg.Event{Kind: pydbg.Stopped, Reason: "step", File: "a.py", Line: 9})
	if u := b.PushConsoleOutput("(py) "); u.Stopped == nil || u.Stopped.Line != 9 {
		t.Errorf("late event: %+v", u)
	}

	// Running alone.
	b.enqueue(pydbg.Event{Kind: pydbg.Running})
	if u := b.PushConsoleOutput(""); u.State != debugger.StateRunning || u.Stopped != nil {
		t.Errorf("running: %+v", u)
	}
}

func TestPYExit(t *testing.T) {
	b := &PYBackend{}
	b.resuming.Store(true)
	b.enqueue(pydbg.Event{Kind: pydbg.Exited})
	if b.resuming.Load() {
		t.Error("exit must clear resuming")
	}
	u := b.PushConsoleOutput("Process has exited with status 0\n(py) ")
	if !u.InferiorExited || u.Stopped != nil {
		t.Errorf("exit: %+v", u)
	}

	// A stop that carries an exit-style reason is an exit, never a refreshing stop.
	b.enqueue(pydbg.Event{Kind: pydbg.Stopped, Reason: "exited-normally"})
	u = b.PushConsoleOutput("")
	if !u.InferiorExited || u.Stopped != nil {
		t.Errorf("exit reason: %+v", u)
	}

	// Restart: exit then running then entry stop ends as a stop.
	b.enqueue(pydbg.Event{Kind: pydbg.Exited})
	b.enqueue(pydbg.Event{Kind: pydbg.Running})
	b.enqueue(pydbg.Event{Kind: pydbg.Stopped, Reason: "entry", File: "a.py", Line: 1})
	u = b.PushConsoleOutput("")
	if u.Stopped == nil || u.InferiorExited {
		t.Errorf("restart: %+v", u)
	}
}

func TestPYStaticPolicy(t *testing.T) {
	b := NewPY(nil)
	if b.Kind() != PY || b.PromptToken() != "(py)" || b.DefaultBreakMain() != "" ||
		b.SelectFrameCmd(3) != "frame 3" || b.SelectThreadCmd("5") != "thread 5" ||
		b.SetInferiorTTYPath("/dev/pts/9") != ErrNotSupported || b.SetInferiorTTYPath("internal") != nil {
		t.Error("static policy mismatch")
	}
}
