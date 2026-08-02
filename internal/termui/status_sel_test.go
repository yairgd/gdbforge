package termui

import (
	"testing"
	"time"
)

func TestStatusWordBoundsPathComponent(t *testing.T) {
	label := "/home/yair/main.c"
	at := len([]rune("/home/yair/"))
	start, end := statusWordBounds(label, at)
	got := string([]rune(label)[start:end])
	if got != "main.c" {
		t.Fatalf("word=%q want main.c (start=%d end=%d)", got, start, end)
	}
}

func TestStatusBandContains(t *testing.T) {
	r := NewRect(10, 5, 40, 12) // Bottom() = 17
	if !statusBandContains(r, 15, 17) {
		t.Fatal("expected status band hit at Bottom()")
	}
	if statusBandContains(r, 15, 16) {
		t.Fatal("content row must not be status band")
	}
}

func TestStatusDoubleClickCopiesFullLabel(t *testing.T) {
	var copied string
	w := &stubPane{BaseWidget: BaseWidget{PaneName: "/tmp/foo.c"}, id: "code"}
	tree := NewWidgetTree(w)
	tree.SetStatusClipboard(ClipboardIO{Copy: func(s string) { copied = s }})
	leaf := tree.FocusedLeaf()
	tree.geom = map[*Node]layoutGeom{
		leaf: {canvas: Canvas{rect: NewRect(0, 0, 80, 20)}},
	}
	mx := statusLabelPrefixCols + 2
	now := time.Now()
	if tree.noteStatusDoubleClick(leaf, mx, now) {
		t.Fatal("first click must not be double")
	}
	if !tree.noteStatusDoubleClick(leaf, mx, now.Add(50*time.Millisecond)) {
		t.Fatal("second click must be double")
	}
	tree.statusCopyFullLabel(leaf)
	if copied != "/tmp/foo.c" {
		t.Fatalf("copied %q want /tmp/foo.c", copied)
	}
	if !tree.statusSel.hasSel {
		t.Fatal("expected highlight after double-click copy")
	}
}

func TestFocusAtStatusBand(t *testing.T) {
	a := &stubPane{id: "a"}
	b := &stubPane{id: "b"}
	tree := NewWidgetTree(a)
	tree.Split(Vertical, b)
	leaves := CollectLeaves(tree.root)
	tree.geom = map[*Node]layoutGeom{
		leaves[0]: {canvas: Canvas{rect: NewRect(0, 0, 40, 10)}},
		leaves[1]: {canvas: Canvas{rect: NewRect(41, 0, 40, 10)}},
	}
	if !tree.FocusAt(5, 10) {
		t.Fatal("FocusAt status band failed")
	}
}

func TestFindSeparatorOnStatusRowIncludingLabel(t *testing.T) {
	code := &stubPane{BaseWidget: BaseWidget{PaneName: "/tmp/foo.c"}, id: "code"}
	gdb := &stubPane{id: "gdb"}
	tree := NewWidgetTree(code)
	tree.Split(Horizontal, gdb)
	leaves := CollectLeaves(tree.root)
	top := leaves[0]
	root := tree.Root()
	tree.geom = map[*Node]layoutGeom{
		root:      {sepRect: NewRect(0, 10, 80, 1), layoutRect: NewRect(0, 0, 80, 21)},
		top:       {canvas: Canvas{rect: NewRect(0, 0, 80, 10)}},
		leaves[1]: {canvas: Canvas{rect: NewRect(0, 11, 80, 10)}},
	}
	// Whole status row (including name) is a gutter again.
	nameX := statusLabelPrefixCols + 2
	if tree.findSeparator(nameX, 10) != root {
		t.Fatal("status row including label must allow separator drag")
	}
	if !tree.statusClickOnLabel(top, nameX) {
		t.Fatal("name still detected for double-click copy")
	}
}
