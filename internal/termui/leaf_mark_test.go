package termui

import (
	"testing"
)

func TestLeafMarkSetAndGet(t *testing.T) {
	a := &stubPane{id: "a"}
	b := &stubPane{id: "b"}
	tree := NewWidgetTree(a)
	tree.Split(Horizontal, b)
	leafA := tree.FindLeaf(func(w Widget) bool {
		s, ok := w.(*stubPane)
		return ok && s.id == "a"
	})
	leafB := tree.FindLeaf(func(w Widget) bool {
		s, ok := w.(*stubPane)
		return ok && s.id == "b"
	})
	if leafA == nil || leafB == nil {
		t.Fatal("missing leaves")
	}
	tree.SetLeafMark("code", leafA)
	tree.SetLeafMark("gdb", leafB)
	if tree.LeafMark("code") != leafA {
		t.Fatal("code mark")
	}
	if tree.LeafMark("gdb") != leafB {
		t.Fatal("gdb mark")
	}
}

func TestLeafMarkClearsStale(t *testing.T) {
	a := &stubPane{id: "a"}
	b := &stubPane{id: "b"}
	tree := NewWidgetTree(a)
	tree.Split(Horizontal, b)
	leafB := tree.FindLeaf(func(w Widget) bool {
		s, ok := w.(*stubPane)
		return ok && s.id == "b"
	})
	tree.SetLeafMark("gdb", leafB)
	tree.FocusWidget(b)
	_ = tree.DeleteFocus() // removes focused B; return value is not a simple ok flag
	if tree.LeafMark("gdb") != nil {
		t.Fatal("stale mark should clear")
	}
}

func TestLeafMarkNilClears(t *testing.T) {
	a := &stubPane{id: "a"}
	tree := NewWidgetTree(a)
	leaf := tree.FindLeaf(func(w Widget) bool { return w == a })
	tree.SetLeafMark("code", leaf)
	tree.SetLeafMark("code", nil)
	if tree.LeafMark("code") != nil {
		t.Fatal("nil clear")
	}
}

func TestOnlyFocusKeepsFocusedLeaf(t *testing.T) {
	a := &stubPane{id: "a"}
	b := &stubPane{id: "b"}
	c := &stubPane{id: "c"}
	tree := NewWidgetTree(a)
	tree.Split(Horizontal, b)
	tree.FocusWidget(b)
	tree.Split(Vertical, c) // focus stays on b's side; c is new leaf
	tree.FocusWidget(b)
	if !tree.OnlyFocus() {
		t.Fatal("OnlyFocus")
	}
	leaves := CollectLeaves(tree.Root())
	if len(leaves) != 1 {
		t.Fatalf("leaves=%d", len(leaves))
	}
	if tree.FocusedWidget() != b {
		t.Fatalf("focus=%v want b", tree.FocusedWidget())
	}
	if tree.Root().Widget != b {
		t.Fatal("root should be focused leaf")
	}
	// Idempotent when already alone.
	if !tree.OnlyFocus() {
		t.Fatal("OnlyFocus alone")
	}
}
