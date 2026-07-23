package dlv

import (
	"path/filepath"
	"testing"

	"github.com/yairgd/gdbforge/internal/gdb"
)

func TestInputStatePromptAndStop(t *testing.T) {
	st := NewInputState()
	u := st.PushRaw("Type 'help' for list of commands.\n(dlv)\n")
	if !u.PromptReady || u.PromptLine != PromptToken {
		t.Fatalf("prompt: ready=%v line=%q", u.PromptReady, u.PromptLine)
	}
	if len(u.DisplayLines) == 0 {
		t.Fatal("expected display lines for banner")
	}

	u = st.PushRaw("> main.main() ./hello.go:5 (hits goroutine(1):1 total:1) (PC: 0x1234)\n(dlv)\n")
	if u.Stopped == nil {
		t.Fatal("expected stop")
	}
	if filepath.Base(u.Stopped.File) != "hello.go" || u.Stopped.Line != 5 {
		t.Fatalf("stop loc: %+v", u.Stopped)
	}
	if u.Stopped.Func != "main.main" {
		t.Fatalf("func: %q", u.Stopped.Func)
	}
	if !u.PromptReady {
		t.Fatal("expected prompt after stop")
	}
}

func TestInputStateStopWithBreakpointTag(t *testing.T) {
	st := NewInputState()
	u := st.PushRaw("> [Breakpoint 1] main.main() ./hello.go:23 (hits goroutine(1):1 total:1) (PC: 0x499f73)\n(dlv)\n")
	if u.Stopped == nil {
		t.Fatal("expected stop")
	}
	if filepath.Base(u.Stopped.File) != "hello.go" || u.Stopped.Line != 23 {
		t.Fatalf("stop loc: %+v", u.Stopped)
	}
	if u.Stopped.Func != "main.main" {
		t.Fatalf("func: %q", u.Stopped.Func)
	}
}

func TestInputStateBreakpointChanged(t *testing.T) {
	st := NewInputState()
	u := st.PushRaw("Breakpoint 1 set at 0xabc for main.main() ./hello.go:5\n(dlv)\n")
	if !u.BreakpointsChanged {
		t.Fatal("expected BreakpointsChanged")
	}
}

func TestInputStateExited(t *testing.T) {
	st := NewInputState()
	u := st.PushRaw("Process has exited with status 0\n(dlv)\n")
	if !u.InferiorExited {
		t.Fatal("expected InferiorExited")
	}
	if u.Stopped == nil || u.Stopped.Reason != "exited-normally" {
		t.Fatalf("stopped: %+v", u.Stopped)
	}
	if gdb.StopNeedsUIRefresh(u.Stopped) {
		t.Fatal("exited stop should not need UI refresh")
	}
}

func TestMapBreakCmd(t *testing.T) {
	tests := []struct{ in, want string }{
		{"-break-delete 3", "clear 3"},
		{"disable 2", "clear 2"},
		{"clear 4", "clear 4"},
		{"clear main.go:10", "clearall main.go:10"},
		{"break main.go:10", "break main.go:10"},
	}
	for _, tc := range tests {
		if got := MapBreakCmd(tc.in); got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseBreakpoints(t *testing.T) {
	raw := `Breakpoint runtime-fatal-throw (enabled) at 0x46dce4 for (multiple functions)() <multiple locations>:0 (0)
Breakpoint unrecovered-panic (enabled) at 0x43b3a4 for runtime.fatalpanic() /usr/lib/go/src/runtime/panic.go:1298 (0)
Breakpoint 1 (enabled) at 0x499f7a for main.main() ./hello.go:24 (0)
Breakpoint 2 (enabled) at 0x499fbb for main.main() ./hello.go:25 (0)
(dlv)
`
	items := ParseBreakpoints(raw)
	if len(items) != 2 {
		t.Fatalf("got %d items: %+v", len(items), items)
	}
	if items[0].Number != 1 || items[0].Line != 24 || filepath.Base(items[0].File) != "hello.go" {
		t.Fatalf("item0: %+v", items[0])
	}
	if items[1].Number != 2 || items[1].Line != 25 {
		t.Fatalf("item1: %+v", items[1])
	}
}

func TestParseBreakpointsSetNotify(t *testing.T) {
	raw := "Breakpoint 1 set at 0xabc for main.main() ./hello.go:5\n(dlv)\n"
	items := ParseBreakpoints(raw)
	if len(items) != 1 || items[0].Line != 5 {
		t.Fatalf("got %+v", items)
	}
}

func TestFrameNavTargetLevel(t *testing.T) {
	level, ok := FrameNavTargetLevel("frame 2", 0)
	if !ok || level != 2 {
		t.Fatalf("frame 2: got %d %v", level, ok)
	}
	level, ok = FrameNavTargetLevel("up", 1)
	if !ok || level != 2 {
		t.Fatalf("up: got %d %v", level, ok)
	}
	level, ok = FrameNavTargetLevel("up 2", 1)
	if !ok || level != 3 {
		t.Fatalf("up 2: got %d %v", level, ok)
	}
	level, ok = FrameNavTargetLevel("down", 2)
	if !ok || level != 1 {
		t.Fatalf("down: got %d %v", level, ok)
	}
	level, ok = FrameNavTargetLevel("down 5", 2)
	if !ok || level != 0 {
		t.Fatalf("down 5: got %d %v", level, ok)
	}
	if _, ok := FrameNavTargetLevel("break main", 0); ok {
		t.Fatal("break should not be frame nav")
	}
}

func TestParseStack(t *testing.T) {
	raw := " 0  0x1234 in main.main\n" +
		"    at ./hello.go:5\n" +
		" 1  0x5678 in runtime.main\n" +
		"    at /usr/lib/go/src/runtime/proc.go:250\n" +
		"(dlv)\n"
	frames := ParseStack(raw)
	if len(frames) != 2 {
		t.Fatalf("got %d frames: %+v", len(frames), frames)
	}
	if frames[0].Func != "main.main" || filepath.Base(frames[0].File) != "hello.go" || frames[0].Line != 5 {
		t.Fatalf("frame0: %+v", frames[0])
	}
}

func TestParseStackWithANSI(t *testing.T) {
	raw := "0  0x0000000000499f73 in \x1b[1mmain.main\x1b[0m\x1b[m\n" +
		"   at ./\x1b[1mhello.go:23\x1b[0m\x1b[m\n" +
		"(dlv)\n"
	frames := ParseStack(raw)
	if len(frames) != 1 {
		t.Fatalf("got %d frames: %+v", len(frames), frames)
	}
	if frames[0].Func != "main.main" || filepath.Base(frames[0].File) != "hello.go" || frames[0].Line != 23 {
		t.Fatalf("frame: %+v", frames[0])
	}
}

func TestParseGoroutines(t *testing.T) {
	raw := `* Goroutine 1 - User: ./hello.go:5 main.main (0) (thread 123)
  Goroutine 2 - User: /usr/lib/go/src/runtime/proc.go:1 runtime.gopark (0)
(dlv)
`
	items := ParseGoroutines(raw)
	if len(items) < 1 {
		t.Fatalf("got %+v", items)
	}
	if !items[0].Current || items[0].ID != "1" {
		t.Fatalf("current: %+v", items[0])
	}
}
