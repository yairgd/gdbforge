package gdb

import (
	"reflect"
	"strings"
	"testing"
)

func TestPushRawStreamsCompleteLinesImmediately(t *testing.T) {
	st := NewGdbInputState()

	// Incomplete first chunk — must not produce display yet.
	u := st.PushRaw("~\"$1 = ")
	if len(u.DisplayLines) != 0 {
		t.Fatalf("incomplete line produced %v", u.DisplayLines)
	}

	u = st.PushRaw("42\\n\"\n^done\n(gdb)\n")
	want := []string{"$1 = 42"}
	if !reflect.DeepEqual(u.DisplayLines, want) {
		t.Fatalf("display=%v want=%v", u.DisplayLines, want)
	}
	if !u.PromptReady {
		t.Fatal("expected PromptReady")
	}
	if u.State != Done {
		t.Fatalf("state=%v want Done", u.State)
	}
}

func TestPushRawErrorSurfacesMsg(t *testing.T) {
	st := NewGdbInputState()
	u := st.PushRaw("&\"bad\\n\"\n^error,msg=\"Undefined command: \\\"bad\\\".\"\n(gdb)\n")
	if len(u.DisplayLines) != 1 || u.DisplayLines[0] != `Undefined command: "bad".` {
		t.Fatalf("display=%v", u.DisplayLines)
	}
	if u.State != Error {
		t.Fatalf("state=%v want Error", u.State)
	}
}

func TestPushRawStopped(t *testing.T) {
	st := NewGdbInputState()
	u := st.PushRaw(`*stopped,reason="breakpoint-hit",thread-id="1",frame={fullname="/tmp/hello.c",line="12"}` + "\n")
	if u.Stopped == nil || u.Stopped.Reason != "breakpoint-hit" || u.Stopped.ThreadId != "1" {
		t.Fatalf("stopped=%+v", u.Stopped)
	}
	if u.Stopped.File != "/tmp/hello.c" || u.Stopped.Line != 12 {
		t.Fatalf("file/line=%q %d", u.Stopped.File, u.Stopped.Line)
	}
}

func TestPushRawBreakpointNotify(t *testing.T) {
	st := NewGdbInputState()
	u := st.PushRaw(`=breakpoint-created,bkpt={number="1",type="breakpoint"}` + "\n")
	if !u.BreakpointsChanged {
		t.Fatal("expected BreakpointsChanged for =breakpoint-created")
	}
	u = st.PushRaw(`=breakpoint-modified,bkpt={number="1",enabled="n"}` + "\n")
	if u.BreakpointsChanged {
		t.Fatal("hit-count/attr modified must not trigger break-list refresh")
	}
	u = st.PushRaw(`=breakpoint-deleted,id="1"` + "\n")
	if !u.BreakpointsChanged {
		t.Fatal("expected BreakpointsChanged for =breakpoint-deleted")
	}
}

func TestPushRawThreadGroupLifecycle(t *testing.T) {
	st := NewGdbInputState()
	u := st.PushRaw(`=thread-group-started,id="i1",pid="24193"` + "\n")
	if u.InferiorPID != "24193" {
		t.Fatalf("pid=%q", u.InferiorPID)
	}
	u = st.PushRaw(`=thread-group-exited,id="i1"` + "\n")
	if !u.InferiorExited {
		t.Fatal("expected InferiorExited")
	}
}

func TestPushRawTargetStreamSeparate(t *testing.T) {
	st := NewGdbInputState()
	u := st.PushRaw("@\"hello\\n\"\n~\"(gdb) \\n\"\n")
	if len(u.TargetLines) != 1 || u.TargetLines[0] != "hello" {
		t.Fatalf("TargetLines=%v", u.TargetLines)
	}
	if len(u.DisplayLines) != 1 || u.DisplayLines[0] != "(gdb) " {
		t.Fatalf("DisplayLines=%v (console ~ must stay separate from @)", u.DisplayLines)
	}
}

func TestPushRawCtrlCPrefixedConsoleStream(t *testing.T) {
	st := NewGdbInputState()
	// GDB echoes Ctrl-C on the PTY, glued to the first ~ console record.
	raw := "\x03~\"\\nProgram\"\n~\" received signal SIGINT, Interrupt.\\n\"\n" +
		`*stopped,reason="signal-received",signal-name="SIGINT",signal-meaning="Interrupt"` + "\n(gdb)\n"
	u := st.PushRaw(raw)
	if u.Stopped == nil || u.Stopped.Reason != "signal-received" {
		t.Fatalf("stopped=%+v", u.Stopped)
	}
	joined := strings.Join(u.DisplayLines, "")
	if !strings.Contains(joined, "Program") || !strings.Contains(joined, "SIGINT") {
		t.Fatalf("expected full signal message in display, got %q", u.DisplayLines)
	}
	if !u.PromptReady {
		t.Fatal("expected PromptReady")
	}
}

func TestPushRawSignalReceivedSynthesizesWhenNoConsoleStream(t *testing.T) {
	st := NewGdbInputState()
	u := st.PushRaw(`*stopped,reason="signal-received",signal-name="SIGINT",signal-meaning="Interrupt"` + "\n(gdb)\n")
	if len(u.DisplayLines) != 1 || u.DisplayLines[0] != "Program received signal SIGINT, Interrupt." {
		t.Fatalf("display=%q", u.DisplayLines)
	}
}

func TestPushRawCtrlCQuitLogStream(t *testing.T) {
	st := NewGdbInputState()
	// Stock GDB: ^C glued onto &"Quit\n"
	u := st.PushRaw("\x03&\"Quit\\n\"\n(gdb)\n")
	if len(u.DisplayLines) != 1 || u.DisplayLines[0] != "Quit" {
		t.Fatalf("display=%q", u.DisplayLines)
	}
	if !u.PromptReady {
		t.Fatal("expected PromptReady")
	}

	st = NewGdbInputState()
	// Custom gdbinit may prefix an emoji.
	u = st.PushRaw("&\"\\342\\235\\214\\357\\270\\217 Quit\\n\"\n(gdb)\n")
	if len(u.DisplayLines) != 1 || !strings.Contains(u.DisplayLines[0], "Quit") {
		t.Fatalf("display=%q", u.DisplayLines)
	}
}

func TestPushRawLogStreamSkipsCommandEcho(t *testing.T) {
	st := NewGdbInputState()
	u := st.PushRaw("&\"break main\\n\"\n^done\n(gdb)\n")
	if len(u.DisplayLines) != 0 {
		t.Fatalf("command echo should stay hidden, got %q", u.DisplayLines)
	}
}

