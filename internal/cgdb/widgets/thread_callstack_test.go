package widgets

import (
	"testing"

	"github.com/yairgd/cgdb-go/internal/mcp"
)

func TestThreadWidgetSetItems(t *testing.T) {
	w := NewThreadWidget()
	if got := w.LinesForTest(); len(got) != 1 || got[0] != "no threads" {
		t.Fatalf("empty=%v", got)
	}
	w.SetItems([]mcp.ThreadInfo{
		{ID: "1", State: "stopped", File: "/tmp/a.c", Line: 10, Current: true},
		{ID: "2", State: "running", File: "b.c", Line: 2},
	})
	lines := w.LinesForTest()
	if len(lines) != 2 || lines[0] != "1  stopped  a.c:10" {
		t.Fatalf("lines=%v", lines)
	}
}

func TestCallStackWidgetSetItems(t *testing.T) {
	w := NewCallStackWidget()
	if got := w.LinesForTest(); len(got) != 1 || got[0] != "no frames" {
		t.Fatalf("empty=%v", got)
	}
	w.SetItems([]mcp.StackFrame{
		{Level: 0, Func: "main", File: "/tmp/hello.c", Line: 12},
		{Level: 1, Func: "start", File: "crt.c", Line: 3},
	})
	lines := w.LinesForTest()
	if len(lines) != 2 || lines[0] != "0  main  hello.c:12" {
		t.Fatalf("lines=%v", lines)
	}
}
