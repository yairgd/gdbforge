package termui

import (
	"math"
	"testing"

	"github.com/yairgd/cgdb-go/internal/platform"
)

func TestNewTabDefaultDebugLayoutLeftRatio(t *testing.T) {
	mk := func(id string) Widget { return &stubPane{id: id} }
	ratios := platform.DefaultLayoutRatios{
		Left:        2.0 / 3.0,
		Output:      1.0 / 2.0,
		BottomFirst: 1.0 / 3.0,
	}
	tw := NewTabDefaultDebugLayout(
		"dbg",
		mk("code"), mk("gdb"), mk("out"), mk("bp"), mk("th"), mk("cs"),
		ratios,
	)
	tree := tw.ActiveTree()
	if tree == nil {
		t.Fatal("nil tree")
	}
	root := tree.Root()
	if root == nil || root.Type != NodeSplit || root.Dir != Vertical {
		t.Fatal("expected outer vertical split")
	}
	if math.Abs(root.Ratio-ratios.Left) > 1e-9 {
		t.Fatalf("left ratio=%v want %v", root.Ratio, ratios.Left)
	}
	right := root.Second
	if right == nil || right.Type != NodeSplit || right.Dir != Horizontal {
		t.Fatal("expected right horizontal Output split")
	}
	if math.Abs(right.Ratio-ratios.Output) > 1e-9 {
		t.Fatalf("output ratio=%v want %v", right.Ratio, ratios.Output)
	}
	bottom := right.Second
	if bottom == nil || bottom.Type != NodeSplit {
		t.Fatal("expected bottom BP stack")
	}
	if math.Abs(bottom.Ratio-ratios.BottomFirst) > 1e-9 {
		t.Fatalf("bp ratio=%v want %v", bottom.Ratio, ratios.BottomFirst)
	}
}

func TestSetEqualAlwaysDoesNotWipeRatio(t *testing.T) {
	mk := func(id string) Widget { return &stubPane{id: id} }
	ratios := platform.DefaultLayoutRatios{
		Left: 2.0 / 3.0, Output: 0.5, BottomFirst: 1.0 / 3.0,
	}
	tw := NewTabDefaultDebugLayout(
		"dbg",
		mk("code"), mk("gdb"), mk("out"), mk("bp"), mk("th"), mk("cs"),
		ratios,
	)
	tree := tw.ActiveTree()
	tree.SetEqualAlways(true)
	if math.Abs(tree.Root().Ratio-ratios.Left) > 1e-9 {
		t.Fatalf("SetEqualAlways wiped ratio=%v", tree.Root().Ratio)
	}
}
