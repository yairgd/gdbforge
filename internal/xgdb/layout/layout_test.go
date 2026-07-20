package layout

import (
	"math"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbx/internal/platform"
	"github.com/yairgd/gdbx/internal/termui"
)

type stubPane struct {
	termui.BaseWidget
	id string
}

func (s *stubPane) HandleEvent(tcell.Event) {}
func (s *stubPane) Draw(termui.Canvas)      {}

func stubPanes() Panes {
	mk := func(id string) termui.Widget { return &stubPane{id: id} }
	return Panes{
		Code: mk("code"), GDB: mk("gdb"), Output: mk("out"),
		Breakpoints: mk("bp"), Threads: mk("th"), Callstack: mk("cs"),
	}
}

func TestBuildDefaultRatios(t *testing.T) {
	ratios := platform.DefaultLayoutRatios{
		Left: 2.0 / 3.0, Output: 1.0 / 2.0, BottomFirst: 1.0 / 3.0,
	}
	tw := BuildDefault("dbg", stubPanes(), ratios)
	root := tw.ActiveTree().Root()
	if root == nil || root.Type != termui.NodeSplit || root.Dir != termui.Vertical {
		t.Fatal("expected outer vertical split")
	}
	if math.Abs(root.Ratio-ratios.Left) > 1e-9 {
		t.Fatalf("left ratio=%v want %v", root.Ratio, ratios.Left)
	}
	right := root.Second
	if right == nil || right.Dir != termui.Horizontal {
		t.Fatal("expected right horizontal Output split")
	}
	if math.Abs(right.Ratio-ratios.Output) > 1e-9 {
		t.Fatalf("output ratio=%v want %v", right.Ratio, ratios.Output)
	}
	bottom := right.Second
	if bottom == nil || bottom.Dir != termui.Horizontal {
		t.Fatal("expected bottom BP stack")
	}
	if math.Abs(bottom.Ratio-ratios.BottomFirst) > 1e-9 {
		t.Fatalf("bp ratio=%v want %v", bottom.Ratio, ratios.BottomFirst)
	}
}

func TestBuildDefaultSetEqualAlwaysKeepsRatio(t *testing.T) {
	ratios := platform.DefaultLayoutRatios{
		Left: 2.0 / 3.0, Output: 0.5, BottomFirst: 1.0 / 3.0,
	}
	tree := BuildDefault("dbg", stubPanes(), ratios).ActiveTree()
	tree.SetEqualAlways(true)
	if math.Abs(tree.Root().Ratio-ratios.Left) > 1e-9 {
		t.Fatalf("SetEqualAlways wiped ratio=%v", tree.Root().Ratio)
	}
}

func TestBuildPanelsStructure(t *testing.T) {
	tw := BuildPanels("panels", stubPanes())
	root := tw.ActiveTree().Root()
	if root == nil || root.Dir != termui.Vertical {
		t.Fatal("expected outer vertical")
	}
	if math.Abs(root.Ratio-panelsLeftRatio) > 1e-9 {
		t.Fatalf("left=%v want %v", root.Ratio, panelsLeftRatio)
	}
	right := root.Second
	if right == nil || right.Dir != termui.Horizontal {
		t.Fatal("expected right Output over bottom")
	}
	if math.Abs(right.Ratio-panelsOutputRatio) > 1e-9 {
		t.Fatalf("output=%v want %v", right.Ratio, panelsOutputRatio)
	}
	bottom := right.Second
	if bottom == nil || bottom.Dir != termui.Horizontal {
		t.Fatal("expected bottom (Threads|CS) over BP")
	}
	if math.Abs(bottom.Ratio-panelsThreadsCallstackRatio) > 1e-9 {
		t.Fatalf("threads|cs ratio=%v want %v", bottom.Ratio, panelsThreadsCallstackRatio)
	}
	if bottom.Second == nil || bottom.Second.Type != termui.NodeLeaf {
		t.Fatal("expected Breakpoints leaf under pair")
	}
	pair := bottom.First
	if pair == nil || pair.Dir != termui.Vertical {
		t.Fatal("expected Threads|Callstack vertical split")
	}
	if pair.First == nil || pair.First.Type != termui.NodeLeaf {
		t.Fatal("expected Threads leaf on left")
	}
	if pair.Second == nil || pair.Second.Type != termui.NodeLeaf {
		t.Fatal("expected Callstack leaf on right")
	}
}

func TestBuildClassicStructure(t *testing.T) {
	tw := BuildClassic("classic", stubPanes())
	root := tw.ActiveTree().Root()
	if root == nil || root.Type != termui.NodeSplit || root.Dir != termui.Horizontal {
		t.Fatal("expected single horizontal Code/GDB")
	}
	if root.First == nil || root.First.Type != termui.NodeLeaf {
		t.Fatal("expected Code leaf")
	}
	if root.Second == nil || root.Second.Type != termui.NodeLeaf {
		t.Fatal("expected GDB leaf")
	}
	// No nested Output column.
	if root.First.Type == termui.NodeSplit || root.Second.Type == termui.NodeSplit {
		t.Fatal("classic must not nest further splits")
	}
	leaves := termui.CollectLeaves(root)
	if len(leaves) != 2 {
		t.Fatalf("leaves=%d want 2", len(leaves))
	}
}
