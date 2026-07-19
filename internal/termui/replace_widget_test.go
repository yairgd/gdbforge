package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

type stubPane struct {
	BaseWidget
	id string
}

func (s *stubPane) HandleEvent(ev tcell.Event) {}
func (s *stubPane) Draw(c Canvas)              {}

func TestReplaceFocusedWidget(t *testing.T) {
	a := &stubPane{id: "a"}
	b := &stubPane{id: "b"}
	tree := NewWidgetTree(a)

	if tree.FocusedWidget() != a {
		t.Fatal("expected initial widget A")
	}
	if !tree.ReplaceFocusedWidget(b) {
		t.Fatal("replace failed")
	}
	if tree.FocusedWidget() != b {
		t.Fatal("expected widget B after replace")
	}
	leaf := tree.FocusedLeaf()
	if leaf == nil || leaf.GetWidget() != b {
		t.Fatal("leaf GetWidget mismatch")
	}
	// Layout identity: still a single leaf root.
	if tree.root != leaf || tree.root.Type != NodeLeaf {
		t.Fatal("tree structure changed")
	}
}

func TestReplaceFocusedWidgetNil(t *testing.T) {
	a := &stubPane{id: "a"}
	tree := NewWidgetTree(a)
	if tree.ReplaceFocusedWidget(nil) {
		t.Fatal("nil widget should fail")
	}
	if tree.FocusedWidget() != a {
		t.Fatal("widget should be unchanged")
	}
}

func TestTopLeftLeafAndFindLeaf(t *testing.T) {
	code := &stubPane{id: "code"}
	gdb := &stubPane{id: "gdb"}
	out := &stubPane{id: "out"}
	tree := NewWidgetTree(code)
	tree.SetEqualAlways(true)
	tree.Split(Vertical, out)
	tree.FocusWidget(code)
	tree.Split(Horizontal, gdb)

	topLeft := tree.TopLeftLeaf()
	if topLeft == nil || topLeft.GetWidget() != code {
		t.Fatalf("top-left=%v want code", topLeft)
	}
	found := tree.FindLeaf(func(w Widget) bool {
		s, ok := w.(*stubPane)
		return ok && s.id == "gdb"
	})
	if found == nil || found.GetWidget() != gdb {
		t.Fatal("FindLeaf gdb")
	}
	if !tree.FocusLeaf(found) || tree.FocusedWidget() != gdb {
		t.Fatal("FocusLeaf gdb")
	}
}
