package models

import (
	"testing"
)

func TestBreakpointListMergeKeepsDisabled(t *testing.T) {
	var b BreakpointList
	b.MergeFromGDB([]BreakInfo{
		{Number: 1, Enabled: true, File: "/tmp/a.c", Line: 10},
	})
	intent, ok := b.ToggleEnableAt(0)
	if !ok || intent.Kind != IntentDeleteByNumber || intent.Number != 1 {
		t.Fatalf("toggle disable intent=%+v ok=%v", intent, ok)
	}
	b.MergeFromGDB(nil)
	if len(b.Items()) != 1 || b.Items()[0].Enabled {
		t.Fatalf("disabled lost: %v", b.Items())
	}
	intent, ok = b.ToggleEnableAt(0)
	if !ok || intent.Kind != IntentInsert || intent.File != "/tmp/a.c" || intent.Line != 10 {
		t.Fatalf("re-enable intent=%+v", intent)
	}
}

func TestBreakpointListToggleInsertClear(t *testing.T) {
	var b BreakpointList
	intent, ok := b.ToggleInsertClear("/tmp/a.c", 5)
	if !ok || intent.Kind != IntentInsert || intent.File != "/tmp/a.c" || intent.Line != 5 {
		t.Fatalf("insert=%+v ok=%v", intent, ok)
	}
	if !b.HasEnabledAt("/tmp/a.c", 5) {
		t.Fatal("expected enabled")
	}
	intent, ok = b.ToggleInsertClear("/tmp/a.c", 5)
	if !ok || intent.Kind != IntentClear || intent.File != "/tmp/a.c" || intent.Line != 5 {
		t.Fatalf("clear=%+v", intent)
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
	intent, ok := b.DeleteAt(0)
	if !ok || intent.Kind != IntentDeleteByNumber || intent.Number != 3 || b.Len() != 0 {
		t.Fatalf("intent=%+v len=%d", intent, b.Len())
	}
}

func TestBreakpointListToggleInsertClearAddr(t *testing.T) {
	var b BreakpointList
	intent, ok := b.ToggleInsertClearAddr("0x401126")
	if !ok || intent.Kind != IntentInsert || intent.Addr != "0x401126" {
		t.Fatalf("insert=%+v ok=%v", intent, ok)
	}
	if !b.HasEnabledAtAddr("0x401126") {
		t.Fatal("expected enabled")
	}
	intent, ok = b.ToggleInsertClearAddr("0x401126")
	if !ok || intent.Kind != IntentClear || intent.Addr != "0x401126" {
		t.Fatalf("clear=%+v", intent)
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
