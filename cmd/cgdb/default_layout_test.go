package main

import (
	"math"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

type layoutStubPane struct {
	termui.BaseWidget
	id string
}

func (s *layoutStubPane) HandleEvent(tcell.Event) {}
func (s *layoutStubPane) Draw(termui.Canvas)      {}

func TestNewTabDefaultDebugLayoutLeftRatio(t *testing.T) {
	mk := func(id string) termui.Widget { return &layoutStubPane{id: id} }
	ratios := platform.DefaultLayoutRatios{
		Left:        2.0 / 3.0,
		Output:      1.0 / 2.0,
		BottomFirst: 1.0 / 3.0,
	}
	tw := newTabDefaultDebugLayout(
		"dbg",
		mk("code"), mk("gdb"), mk("out"), mk("bp"), mk("th"), mk("cs"),
		ratios,
	)
	tree := tw.ActiveTree()
	if tree == nil {
		t.Fatal("nil tree")
	}
	root := tree.Root()
	if root == nil || root.Type != termui.NodeSplit || root.Dir != termui.Vertical {
		t.Fatal("expected outer vertical split")
	}
	if math.Abs(root.Ratio-ratios.Left) > 1e-9 {
		t.Fatalf("left ratio=%v want %v", root.Ratio, ratios.Left)
	}
	right := root.Second
	if right == nil || right.Type != termui.NodeSplit || right.Dir != termui.Horizontal {
		t.Fatal("expected right horizontal Output split")
	}
	if math.Abs(right.Ratio-ratios.Output) > 1e-9 {
		t.Fatalf("output ratio=%v want %v", right.Ratio, ratios.Output)
	}
	bottom := right.Second
	if bottom == nil || bottom.Type != termui.NodeSplit {
		t.Fatal("expected bottom BP stack")
	}
	if math.Abs(bottom.Ratio-ratios.BottomFirst) > 1e-9 {
		t.Fatalf("bp ratio=%v want %v", bottom.Ratio, ratios.BottomFirst)
	}
}

func TestSetEqualAlwaysDoesNotWipeRatio(t *testing.T) {
	mk := func(id string) termui.Widget { return &layoutStubPane{id: id} }
	ratios := platform.DefaultLayoutRatios{
		Left: 2.0 / 3.0, Output: 0.5, BottomFirst: 1.0 / 3.0,
	}
	tw := newTabDefaultDebugLayout(
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
