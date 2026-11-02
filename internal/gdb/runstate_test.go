package gdb

import (
	"context"
	"testing"
	"time"
)

func TestRunTrackerFollowsAsyncRecords(t *testing.T) {
	tr := &RunTracker{change: make(chan struct{})}
	// A prompt after ^running must not read as stopped; chunks split mid-line.
	tr.Feed("5^runn")
	tr.Feed("ing\n*running,thread-id=\"all\"\n(gdb) \n")
	if got := tr.State(); got != "running" {
		t.Fatalf("state=%q, want running", got)
	}
	tr.Feed("6^done,threads=[]\n(gdb) \n") // a query reply while running changes nothing
	if got := tr.State(); got != "running" {
		t.Fatalf("state=%q after query reply, want running", got)
	}
	seq := tr.Seq()
	tr.Feed(`*stopped,reason="breakpoint-hit",disp="keep",bkptno="3",frame={addr="0x0000000000401136",func="main",args=[],file="hello.c",fullname="/tmp/hello.c",line="7"},thread-id="1"` + "\n(gdb) \n")
	state, stop := tr.Snapshot()
	if state != "stopped" || stop.BkptNo != 3 || stop.Func != "main" || stop.Line != 7 ||
		stop.Addr != "0x0000000000401136" || stop.File != "/tmp/hello.c" || stop.Reason != "breakpoint-hit" {
		t.Fatalf("state=%q stop=%+v", state, stop)
	}
	if _, ok := tr.WaitStop(context.Background(), seq, time.Second); !ok {
		t.Fatal("WaitStop missed a stop newer than seq")
	}
	if _, ok := tr.WaitStop(context.Background(), tr.Seq(), 20*time.Millisecond); ok {
		t.Fatal("WaitStop must time out when no newer stop arrives")
	}
}

func TestRunTrackerWatchpointAndExit(t *testing.T) {
	tr := &RunTracker{change: make(chan struct{})}
	tr.Feed("*running,thread-id=\"all\"\n")
	tr.Feed(`*stopped,reason="watchpoint-trigger",wpt={number="4",exp="x"},value={old="1",new="2"},frame={addr="0x10",func="f",line="2"}` + "\n")
	if _, s := tr.Snapshot(); s.BkptNo != 4 {
		t.Fatalf("watchpoint number=%d, want 4", s.BkptNo)
	}
	tr.Feed("*running,thread-id=\"all\"\n")
	tr.Feed(`*stopped,reason="exited-normally"` + "\n")
	if got := tr.State(); got != "disconnected" {
		t.Fatalf("state=%q after exit, want disconnected", got)
	}
}

func TestRunTrackerDisconnectWakesWaiter(t *testing.T) {
	tr := &RunTracker{change: make(chan struct{})}
	tr.Feed("*running\n")
	seq := tr.Seq()
	go func() {
		time.Sleep(20 * time.Millisecond)
		tr.Feed("=thread-group-exited,id=\"i1\"\n")
	}()
	s, ok := tr.WaitStop(context.Background(), seq, 2*time.Second)
	if !ok || s.Reason != "exited" || tr.State() != "disconnected" {
		t.Fatalf("ok=%v stop=%+v state=%q", ok, s, tr.State())
	}
}

func TestRunTrackerMarkRunningAndSeed(t *testing.T) {
	tr := &RunTracker{change: make(chan struct{})}
	tr.Seed("stopped", StopInfo{Func: "main"})
	tr.Seed("running", StopInfo{}) // ignored: a live state is never replaced
	if tr.State() != "stopped" {
		t.Fatalf("seed must not replace a live state, got %q", tr.State())
	}
	seq := tr.Seq()
	tr.MarkRunning(seq)
	if tr.State() != "running" {
		t.Fatal("MarkRunning with a current seq must set running")
	}
	tr.Feed(`*stopped,reason="end-stepping-range",frame={addr="0x1",func="m",line="1"}` + "\n")
	tr.MarkRunning(seq) // a stop arrived since: must not override it
	if tr.State() != "stopped" {
		t.Fatalf("MarkRunning with an old seq must not override a newer stop, got %q", tr.State())
	}
}

func TestParseThreadInfo(t *testing.T) {
	st, _ := ParseThreadInfo(`^done,threads=[{id="1",target-id="Thread 1",name="sleep",state="running",core="3"}],current-thread-id="1"`)
	if st != "running" {
		t.Fatalf("running: %q", st)
	}
	st, stop := ParseThreadInfo(`^done,threads=[{id="1",target-id="Thread 1",frame={level="0",addr="0x00000100",func="loop",args=[],file="a.c",fullname="/x/a.c",line="9",arch="armv7"},state="stopped"}],current-thread-id="1"`)
	if st != "stopped" || stop.Func != "loop" || stop.Line != 9 || stop.Addr != "0x00000100" || stop.File != "/x/a.c" {
		t.Fatalf("stopped: %q %+v", st, stop)
	}
	if st, _ := ParseThreadInfo(`^done,threads=[]`); st != "disconnected" {
		t.Fatalf("empty: %q", st)
	}
}

func TestPushRawKeepsRunningAcrossPrompt(t *testing.T) {
	m := NewGdbInputState()
	if u := m.PushRaw("5^running\n*running,thread-id=\"all\"\n(gdb)\n"); !u.Running {
		t.Fatal("^running + prompt must leave Running set")
	}
	if u := m.PushRaw("6^done\n(gdb)\n"); !u.Running || !u.PromptReady {
		t.Fatalf("a bare prompt while running must keep Running: %+v", u)
	}
	if u := m.PushRaw("*stopped,reason=\"end-stepping-range\"\n(gdb)\n"); u.Running {
		t.Fatal("*stopped must clear Running")
	}
}

func TestRunTrackerConnectedIsStoppedAndSeedRecoversDisconnected(t *testing.T) {
	tr := &RunTracker{change: make(chan struct{})}
	tr.Feed("=thread-group-exited,id=\"i1\"\n")
	if tr.State() != "disconnected" {
		t.Fatalf("state=%q", tr.State())
	}
	tr.Seed("stopped", StopInfo{Func: "main", Addr: "0x1"}) // thread-info says a target is attached
	if st, s := tr.Snapshot(); st != "stopped" || s.Func != "main" {
		t.Fatalf("seed must recover from disconnected: %q %+v", st, s)
	}
	tr2 := &RunTracker{change: make(chan struct{})}
	tr2.Feed("=thread-group-exited,id=\"i1\"\n^connected\n")
	if tr2.State() != "stopped" {
		t.Fatalf("^connected must read as stopped, got %q", tr2.State())
	}
	tr2.FillLocation(StopInfo{Func: "reset", Addr: "0x0", Line: 3})
	if _, s := tr2.Snapshot(); s.Func != "reset" {
		t.Fatalf("FillLocation: %+v", s)
	}
}
