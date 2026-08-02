package models

import (
	"testing"
)

func TestBreakpointListMergeKeepsDisabled(t *testing.T) {
	var b BreakpointList
	b.MergeFromGDB([]BreakInfo{
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
	b.MergeFromGDB([]BreakInfo{
		{Number: 3, Enabled: true, File: "/tmp/a.c", Line: 5},
	})
	cmd, ok := b.DeleteAt(0)
	if !ok || cmd != "-break-delete 3" || b.Len() != 0 {
		t.Fatalf("cmd=%q len=%d", cmd, b.Len())
	}
}

func TestBreakpointListToggleInsertClearAddr(t *testing.T) {
	var b BreakpointList
	cmd, ok := b.ToggleInsertClearAddr("0x401126")
	if !ok || cmd != "break *0x401126" {
		t.Fatalf("insert=%q ok=%v", cmd, ok)
	}
	if !b.HasEnabledAtAddr("0x401126") {
		t.Fatal("expected enabled")
	}
	cmd, ok = b.ToggleInsertClearAddr("0x401126")
	if !ok || cmd != "clear *0x401126" {
		t.Fatalf("clear=%q", cmd)
	}
	if b.HasEnabledAtAddr("0x401126") {
		t.Fatal("expected cleared")
	}
}

func TestBreakpointListAsmAndCodeAt(t *testing.T) {
	var b BreakpointList
	b.MergeFromGDB([]BreakInfo{
		{Number: 1, Enabled: true, File: "/tmp/a.c", Line: 10, Addr: "0x401126"},
		{Number: 2, Enabled: true, File: "/tmp/a.c", Line: 20},
	})
	if !b.HasAsmAndCodeAt("/tmp/a.c", 10) {
		t.Fatal("expected linked asm+code at line 10")
	}
	if b.HasAsmAndCodeAt("/tmp/a.c", 20) {
		t.Fatal("line 20 has no addr")
	}
	bp, ok := b.SourceAtAddr("0x401126")
	if !ok || bp.File != "/tmp/a.c" || bp.Line != 10 {
		t.Fatalf("SourceAtAddr=%+v ok=%v", bp, ok)
	}
}
