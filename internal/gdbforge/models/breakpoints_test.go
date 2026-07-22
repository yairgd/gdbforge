package models

import (
	"testing"

	"github.com/yairgd/gdbforge/internal/mcp"
)

func TestBreakpointListMergeKeepsDisabled(t *testing.T) {
	var b BreakpointList
	b.MergeFromGDB([]mcp.BreakInfo{
		{Number: 1, Enabled: true, File: "/tmp/a.c", Line: 10},
	})
	cmd, ok := b.ToggleEnableAt(0)
	if !ok || cmd != "-break-delete 1" {
		t.Fatalf("toggle disable cmd=%q ok=%v", cmd, ok)
	}
	b.MergeFromGDB(nil)
	if len(b.Items()) != 1 || b.Items()[0].Enabled {
		t.Fatalf("disabled lost: %v", b.Items())
	}
	cmd, ok = b.ToggleEnableAt(0)
	if !ok || cmd != "break /tmp/a.c:10" {
		t.Fatalf("re-enable cmd=%q", cmd)
	}
}

func TestBreakpointListToggleInsertClear(t *testing.T) {
	var b BreakpointList
	cmd, ok := b.ToggleInsertClear("/tmp/a.c", 5)
	if !ok || cmd != "break a.c:5" {
		t.Fatalf("insert=%q ok=%v", cmd, ok)
	}
	if !b.HasEnabledAt("/tmp/a.c", 5) {
		t.Fatal("expected enabled")
	}
	cmd, ok = b.ToggleInsertClear("/tmp/a.c", 5)
	if !ok || cmd != "clear a.c:5" {
		t.Fatalf("clear=%q", cmd)
	}
	if b.HasEnabledAt("/tmp/a.c", 5) {
		t.Fatal("expected cleared")
	}
}

func TestBreakpointListDelete(t *testing.T) {
	var b BreakpointList
	b.MergeFromGDB([]mcp.BreakInfo{
		{Number: 3, Enabled: true, File: "/tmp/a.c", Line: 5},
	})
	cmd, ok := b.DeleteAt(0)
	if !ok || cmd != "-break-delete 3" || b.Len() != 0 {
		t.Fatalf("cmd=%q len=%d", cmd, b.Len())
	}
}

func TestBreakpointListToggleEnableAtFileLine(t *testing.T) {
	var b BreakpointList
	b.MergeFromGDB([]mcp.BreakInfo{
		{Number: 2, Enabled: true, File: "/tmp/a.c", Line: 10},
	})
	cmd, idx, ok := b.ToggleEnableAtFileLine("/tmp/a.c", 10, false)
	if !ok || idx != 0 || cmd != "-break-delete 2" {
		t.Fatalf("cmd=%q idx=%d", cmd, idx)
	}
	_, _, ok = b.ToggleEnableAtFileLine("/tmp/a.c", 99, false)
	if ok {
		t.Fatal("expected no-op")
	}
}
