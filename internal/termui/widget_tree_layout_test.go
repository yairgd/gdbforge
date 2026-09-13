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

// Shrinking the terminal clamps panes against minPaneCells. BuildLayout used to
// write the clamped sizes back into Node.Ratio, so growing the terminal again
// re-expanded the wrong proportions and the tab no longer matched the layout the
// user selected. Geometry must be a pure function of the ratios and the canvas.
func TestBuildLayoutResizeIsReversible(t *testing.T) {
	// Shape of layout.BuildDefault: Code|Output split, each side split further.
	tree := NewWidgetTree(NewStubWidget("code"))
	tree.Split(Vertical, NewStubWidget("output"))
	tree.FocusWidget(tree.Root().First.Widget)
	tree.Split(Horizontal, NewStubWidget("gdb"))
	tree.FocusWidget(tree.Root().Second.Widget)
	tree.Split(Horizontal, NewStubWidget("threads"))

	tree.Root().Ratio = 0.6
	tree.Root().Second.Ratio = 0.5
	want := map[string]float64{"root": 0.6, "right": 0.5}

	build := func(w, h int) map[string]Rect {
		tree.BuildLayout(NewCanvas(NewGrid(w, h)).WithRect(NewRect(0, 0, w, h)))
		out := map[string]Rect{}
		for node, g := range tree.geom {
			if node.Type == NodeLeaf {
				out[node.Widget.(*stubWidget).PaneName] = g.canvas.Rect()
			}
		}
		return out
	}
	checkRatios := func(when string) {
		if got := tree.Root().Ratio; got != want["root"] {
			t.Errorf("%s: root ratio %v, want %v", when, got, want["root"])
		}
		if got := tree.Root().Second.Ratio; got != want["right"] {
			t.Errorf("%s: right ratio %v, want %v", when, got, want["right"])
		}
	}

	before := build(200, 60)
	checkRatios("large")

	// Small enough that minPaneCells clamps the nested splits.
	build(30, 10)
	checkRatios("small")

	after := build(200, 60)
	checkRatios("restored")

	if len(before) != 4 {
		t.Fatalf("expected 4 panes, got %d", len(before))
	}
	for name, r := range before {
		if after[name] != r {
			t.Errorf("pane %q not restored: %v -> %v", name, r, after[name])
		}
	}
}

// Pane sizes the user set by dragging a separator are proportions, not a
// snapshot: resizing the console must scale them, never restore the ratios the
// named layout was built with.
func TestResizeKeepsDraggedProportions(t *testing.T) {
	tree := NewWidgetTree(NewStubWidget("left"))
	tree.Split(Vertical, NewStubWidget("right"))
	tree.SetEqualAlways(false)
	tree.Root().Ratio = 0.5

	build := func(w, h int) {
		tree.BuildLayout(NewCanvas(NewGrid(w, h)).WithRect(NewRect(0, 0, w, h)))
	}
	leftWidth := func() int {
		for node, g := range tree.geom {
			if node.Type == NodeLeaf && node.Widget.(*stubWidget).PaneName == "left" {
				return g.canvas.Rect().W()
			}
		}
		return -1
	}

	// Drag the separator well off the layout default of 0.5.
	build(100, 40)
	tree.HandleEvent(tcell.NewEventMouse(leftWidth(), 5, tcell.ButtonPrimary, 0))
	tree.HandleEvent(tcell.NewEventMouse(70, 5, tcell.ButtonPrimary, 0))
	tree.HandleEvent(tcell.NewEventMouse(70, 5, tcell.ButtonNone, 0))

	dragged := tree.Root().Ratio
	if dragged < 0.65 {
		t.Fatalf("drag did not take effect: ratio=%v", dragged)
	}

	for _, sz := range [][2]int{{200, 60}, {60, 20}, {140, 45}, {100, 40}} {
		build(sz[0], sz[1])
		if got := tree.Root().Ratio; got != dragged {
			t.Fatalf("%dx%d: ratio drifted %v -> %v", sz[0], sz[1], dragged, got)
		}
		avail := float64(sz[0] - 1)
		want := int(avail * dragged)
		if got := leftWidth(); got != want {
			t.Errorf("%dx%d: left pane %d cells, want %d (proportion %.3f)",
				sz[0], sz[1], got, want, dragged)
		}
	}
}

// A console too small for every split leaves panes without geometry. Those
// panes used to be painted anyway, with the zero-value canvas: gdbforge
// panicked on startup in a 27x8 terminal because CompositeTerminal.Paint
// forwarded it as Resize(0, 0) and xterm-go indexed off a -1 row.
func TestDrawSkipsPanesWithoutGeometry(t *testing.T) {
	names := []string{"code", "output", "gdb", "bps", "threads", "stack"}
	panes := map[string]*recordingWidget{}
	for _, n := range names {
		panes[n] = &recordingWidget{BaseWidget: BaseWidget{PaneName: n}}
	}

	// Shape of layout.BuildDefault.
	tree := NewWidgetTree(panes["code"])
	tree.SetEqualAlways(true)
	tree.Split(Vertical, panes["output"])
	tree.FocusWidget(panes["code"])
	tree.Split(Horizontal, panes["gdb"])
	tree.FocusWidget(panes["output"])
	tree.Split(Horizontal, panes["bps"])
	tree.FocusWidget(panes["bps"])
	tree.Split(Horizontal, panes["threads"])
	tree.FocusWidget(panes["threads"])
	tree.Split(Horizontal, panes["stack"])
	tree.SetEqualAlways(false)
	tree.FocusWidget(panes["gdb"])

	c := NewCanvas(NewGrid(27, 8)).WithRect(NewRect(0, 0, 27, 6))
	tree.BuildLayout(c)
	tree.Draw(c)

	for _, n := range names {
		for _, r := range panes[n].rects {
			if r.W() <= 0 || r.H() <= 0 {
				t.Errorf("pane %q drawn with empty canvas %v", n, r)
			}
		}
	}
	// Splits that cannot fit collapse onto one child rather than being dropped,
	// so the console still shows the focused pane instead of going blank.
	if len(panes["gdb"].rects) == 0 {
		t.Error("focused pane was not drawn")
	}
}

type recordingWidget struct {
	BaseWidget
	rects []Rect
}

func (w *recordingWidget) Draw(c Canvas)           { w.rects = append(w.rects, c.Rect()) }
func (w *recordingWidget) HandleEvent(tcell.Event) {}

type stubWidget struct {
	BaseWidget
}

func NewStubWidget(name string) Widget {
	return &stubWidget{BaseWidget: BaseWidget{PaneName: name}}
}

func (s *stubWidget) Draw(Canvas)             {}
func (s *stubWidget) HandleEvent(tcell.Event) {}
