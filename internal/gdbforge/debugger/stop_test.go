package debugger

import (
	"testing"

	"github.com/yairgd/gdbforge/internal/gdb"
)

func TestStopFromGDB(t *testing.T) {
	stop := StopFromGDB(&gdb.MiStopMsg{
		Reason:   "breakpoint-hit",
		ThreadId: "1",
		File:     "main.c",
		Func:     "main",
		Line:     42,
	})
	if stop == nil || stop.Reason != "breakpoint-hit" || stop.ThreadID != "1" {
		t.Fatalf("unexpected stop: %+v", stop)
	}
	if stop.File != "main.c" || stop.Line != 42 || stop.Func != "main" {
		t.Fatalf("location: %+v", stop.SourceLocation)
	}
}

func TestStopInfoNeedsUIRefresh(t *testing.T) {
	if (&StopInfo{Reason: "exited-normally"}).NeedsUIRefresh() {
		t.Fatal("exit should not refresh")
	}
	if !(&StopInfo{Reason: "breakpoint-hit"}).NeedsUIRefresh() {
		t.Fatal("breakpoint should refresh")
	}
}
