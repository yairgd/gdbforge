package models

import (
	"testing"
)

func TestThreadListSetItems(t *testing.T) {
	var list ThreadList
	list.Set([]ThreadInfo{{ID: "1", Func: "main"}})
	got := list.Items()
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("items=%v", got)
	}
	got[0].ID = "x"
	if list.Items()[0].ID != "1" {
		t.Fatal("Items must copy")
	}
}

func TestCallStackFirstWithFile(t *testing.T) {
	var cs CallStack
	cs.Set([]StackFrame{
		{Level: 0, Func: "foo"},
		{Level: 1, Func: "main", File: "/tmp/a.c", Line: 10},
	})
	fr, ok := cs.FirstWithFile()
	if !ok || fr.File != "/tmp/a.c" || fr.Line != 10 {
		t.Fatalf("got %#v ok=%v", fr, ok)
	}
}
