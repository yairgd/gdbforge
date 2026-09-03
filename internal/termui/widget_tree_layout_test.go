package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestBuildLayoutPreservesRatioOnDegenerateCanvas(t *testing.T) {
	tree := NewWidgetTree(NewStubWidget("a"))
	tree.Split(Horizontal, NewStubWidget("b"))
	tree.root.Ratio = 0.5

	large := NewCanvas(NewGrid(80, 40)).WithRect(NewRect(0, 0, 80, 40))
	tree.BuildLayout(large)
	before := tree.root.Ratio

	// One frame with an impossibly small canvas must not rewrite split ratios.
	tiny := NewCanvas(NewGrid(4, 4)).WithRect(NewRect(0, 0, 4, 4))
	tree.BuildLayout(tiny)
	if tree.root.Ratio != before {
		t.Fatalf("ratio changed %v -> %v on tiny canvas", before, tree.root.Ratio)
	}

	tree.BuildLayout(large)
	if tree.root.Ratio != before {
		t.Fatalf("ratio changed %v -> %v after recovery", before, tree.root.Ratio)
	}
}

type stubWidget struct {
	BaseWidget
}

func NewStubWidget(name string) Widget {
	return &stubWidget{BaseWidget: BaseWidget{PaneName: name}}
}

func (s *stubWidget) Draw(Canvas)              {}
func (s *stubWidget) HandleEvent(tcell.Event) {}
