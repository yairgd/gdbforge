package mcp

import "testing"

func TestParseThreadInfo(t *testing.T) {
	raw := `^done,threads=[{id="1",target-id="Thread 1",name="main",state="stopped",frame={level="0",addr="0x4005",func="main",file="hello.c",fullname="/tmp/hello.c",line="12"}},{id="2",target-id="Thread 2",state="running",frame={level="0",func="worker",file="w.c",line="8"}}],current-thread-id="1"`
	got := ParseThreadInfo(raw)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2: %+v", len(got), got)
	}
	if got[0].ID != "1" || got[0].State != "stopped" || got[0].Line != 12 || !got[0].Current {
		t.Fatalf("first=%+v", got[0])
	}
	if got[0].File != "/tmp/hello.c" || got[0].Func != "main" {
		t.Fatalf("first loc=%+v", got[0])
	}
	if got[1].ID != "2" || got[1].Current {
		t.Fatalf("second=%+v", got[1])
	}
}

func TestParseStackListFrames(t *testing.T) {
	raw := `^done,stack=[frame={level="0",addr="0x4005",func="main",file="hello.c",fullname="/tmp/hello.c",line="12"},frame={level="1",addr="0x4010",func="start",file="crt.c",line="3"}]`
	got := ParseStackListFrames(raw)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	if got[0].Level != 0 || got[0].Func != "main" || got[0].Line != 12 {
		t.Fatalf("frame0=%+v", got[0])
	}
	if got[1].Level != 1 || got[1].Func != "start" {
		t.Fatalf("frame1=%+v", got[1])
	}
}
