package gdb

import (
	"reflect"
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
	if !u.BreakpointsChanged {
		t.Fatal("expected BreakpointsChanged for =breakpoint-modified")
	}
	u = st.PushRaw(`=breakpoint-deleted,id="1"` + "\n")
	if !u.BreakpointsChanged {
		t.Fatal("expected BreakpointsChanged for =breakpoint-deleted")
	}
}

