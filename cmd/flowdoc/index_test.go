package main

import (
	"strings"
	"testing"
)

func TestIndexFindsHandleInsertKey(t *testing.T) {
	root := "../.."
	idx, err := loadFuncIndex(root, []string{"./cmd/gdbforge"})
	if err != nil {
		t.Fatal(err)
	}
	link := chainSpec{
		Symbol: "handleInsertKey",
		Pkg:    "cmd/gdbforge",
		Recv:   "(*DebuggerApp)",
		Name:   "handleInsertKey",
	}
	ref, err := idx.resolve(link)
	if err != nil {
		for _, r := range idx.all {
			if strings.Contains(r.Name, "handleInsert") {
				t.Logf("candidate: pkg=%s recv=%q name=%s file=%s:%d", r.PkgPath, r.Recv, r.Name, r.File, r.Line)
			}
		}
		t.Fatal(err)
	}
	t.Logf("found %s at %s:%d", ref.FullSymbol, ref.File, ref.Line)
}
