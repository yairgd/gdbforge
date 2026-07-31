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
	if u.PromptLine != MIPromptToken {
		t.Fatalf("PromptLine=%q want %q", u.PromptLine, MIPromptToken)
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

func TestPushRawThreadSelectedFrame(t *testing.T) {
	st := NewGdbInputState()
	u := st.PushRaw(`=thread-selected,id="1",frame={level="2",addr="0x400",func="main",args=[],file="hello.c",fullname="/tmp/hello.c",line="10",arch="i386:x86-64"}` + "\n^done\n(gdb)\n")
	if u.FrameSelected == nil {
		t.Fatal("expected FrameSelected from =thread-selected")
	}
	if u.FrameSelected.Level != 2 || u.FrameSelected.File != "/tmp/hello.c" || u.FrameSelected.Line != 10 {
		t.Fatalf("got %+v", u.FrameSelected)
	}
	if u.FrameSelected.Func != "main" || u.FrameSelected.Addr != "0x400" {
		t.Fatalf("got %+v", u.FrameSelected)
	}
	if !u.PromptReady {
		t.Fatal("expected PromptReady")
	}
}

func TestPushRawTargetStreamSeparate(t *testing.T) {
	st := NewGdbInputState()
	u := st.PushRaw("@\"hello\\n\"\n~\"(gdb) \\n\"\n")
	if len(u.TargetLines) != 1 || u.TargetLines[0] != "hello" {
		t.Fatalf("TargetLines=%v", u.TargetLines)
	}
	if len(u.DisplayLines) != 1 || u.DisplayLines[0] != MIPromptLiveHost {
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

func TestPushRawMakeShellRawOutput(t *testing.T) {
	st := NewGdbInputState()
	// GDB make/shell: log-stream echo, then raw child stdout, then ^done.
	u := st.PushRaw("&\"make --version\\n\"\n" +
		"GNU Make 4.4.1\n" +
		"Built for x86_64-pc-linux-gnu\n" +
		"^done\n(gdb)\n")
	want := []string{"GNU Make 4.4.1", "Built for x86_64-pc-linux-gnu"}
	if !reflect.DeepEqual(u.DisplayLines, want) {
		t.Fatalf("display=%v want=%v", u.DisplayLines, want)
	}
	if u.State != Done {
		t.Fatalf("state=%v want Done", u.State)
	}
	if !u.PromptReady {
		t.Fatal("expected PromptReady")
	}
}

func TestPushRawMakeKeepsANSIEscape(t *testing.T) {
	st := NewGdbInputState()
	// gcc -fdiagnostics-color lines often start with ESC[…m — must not strip ESC.
	colored := "\x1b[01m/tmp/foo.c:1: error: boom\x1b[0m"
	u := st.PushRaw(colored + "\n^done\n(gdb)\n")
	if len(u.DisplayLines) != 1 || u.DisplayLines[0] != colored {
		t.Fatalf("display=%q want ESC preserved", u.DisplayLines)
	}
}

func TestPushRawRunningThenStoppedClearsRunningState(t *testing.T) {
	st := NewGdbInputState()
	u := st.PushRaw(
		"^running\n" +
			`*stopped,reason="end-stepping-range",thread-id="1",frame={fullname="/tmp/hello.c",line="12"}` + "\n" +
			"(gdb)\n",
	)
	if u.Stopped == nil {
		t.Fatal("expected Stopped")
	}
	if !u.PromptReady {
		t.Fatal("expected PromptReady")
	}
	if u.State != Done {
		t.Fatalf("state=%v want Done after *stopped (Ctrl-Z InferiorRunning)", u.State)
	}
}

func TestPushRawStoppedThenRunningKeepsRunningState(t *testing.T) {
	// Ctrl-C + break + continue: *stopped then ^running in one chunk.
	st := NewGdbInputState()
	u := st.PushRaw(
		`*stopped,reason="signal-received",signal-name="SIGINT",thread-id="1"` + "\n" +
			"=breakpoint-created,bkpt={number=\"2\"}\n" +
			"^running\n",
	)
	if u.Stopped == nil {
		t.Fatal("expected Stopped from Ctrl-C")
	}
	if !u.BreakpointsChanged {
		t.Fatal("expected breakpoint-created")
	}
	if u.State != Running {
		t.Fatalf("state=%v want Running after trailing ^running", u.State)
	}
}

func TestPushRawHidesDownloadStatusShowsLoadText(t *testing.T) {
	st := NewGdbInputState()
	// load / remote write: MI +download status is protocol noise; console ~
	// and raw "Loading section" lines match native GDB.
	u := st.PushRaw(
		`+download,{section=".init",section-size="12",total-size="68266353"}` + "\n" +
			`~"Loading section .init, size 0xc lma 0x412f4a4\n"` + "\n" +
			"Loading section .data, size 0xa1e4 lma 0x4130f98\n" +
			`+download,{section=".data",section-sent="41444",section-size="41444",total-sent="295079",total-size="68266353"}` + "\n" +
			`~"Transfer rate: 278 KB/sec, 21082 bytes/write.\n"` + "\n" +
			"^done\n(gdb)\n",
	)
	want := []string{
		"Loading section .init, size 0xc lma 0x412f4a4",
		"Loading section .data, size 0xa1e4 lma 0x4130f98",
		"Transfer rate: 278 KB/sec, 21082 bytes/write.",
	}
	if !reflect.DeepEqual(u.DisplayLines, want) {
		t.Fatalf("display=%v want=%v", u.DisplayLines, want)
	}
	for _, ln := range u.DisplayLines {
		if strings.HasPrefix(ln, "+download") {
			t.Fatalf("MI +download must stay hidden, got %q", ln)
		}
	}
}
