package termui

import "testing"

func TestStatusClickOnLabelBounds(t *testing.T) {
	code := &stubPane{BaseWidget: BaseWidget{PaneName: "/tmp/foo.c"}, id: "code"}
	tree := NewWidgetTree(code)
	leaf := tree.FocusedLeaf()
	tree.geom = map[*Node]layoutGeom{
		leaf: {canvas: Canvas{rect: NewRect(0, 0, 80, 10)}},
	}
	if tree.statusClickOnLabel(leaf, 0) {
		t.Fatal("prefix is not the name")
	}
	if !tree.statusClickOnLabel(leaf, statusLabelPrefixCols) {
		t.Fatal("first name column")
	}
	past := statusLabelEndCol(code.StatusLabel())
	if tree.statusClickOnLabel(leaf, past) {
		t.Fatal("past name is not the name")
	}
}
