package main

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/termui"
)

type stubView struct {
	termui.BaseWidget
	id string
}

func (s *stubView) HandleEvent(ev tcell.Event) {}
func (s *stubView) Draw(c termui.Canvas)       {}

// newGdbLeafApp builds a minimal DebuggerApp with Code | GDB side by side and
// the "gdb" leaf mark set on the GDB pane.
func newGdbLeafApp() *DebuggerApp {
	code := &stubView{id: "code"}
	gdb := widgets.NewGDBWidget()
	other := &stubView{id: "other"}
	tab := termui.NewTabTwoHozSplitWins("test", code, gdb)
	a := &DebuggerApp{
		tab:       tab,
		gdbWidget: gdb,
		builtins:  map[string]termui.Widget{"other": other},
	}
	a.tab.FocusWidget(gdb)
	a.tab.SetLeafMark(leafMarkCode, a.tab.FindLeaf(func(w termui.Widget) bool { return w == code }))
	a.tab.SetLeafMark(leafMarkGDB, a.tab.FindLeaf(func(w termui.Widget) bool { return w == gdb }))
	return a
}

func TestSwapFocusedWidgetRefusesGdbLeaf(t *testing.T) {
	a := newGdbLeafApp()
	other := a.builtins["other"]
	gdb := a.gdbWidget

	if !a.isGdbLeaf(a.focusedLeaf()) {
		t.Fatal("expected GDB leaf focused")
	}
	if a.swapFocusedWidget(other) {
		t.Fatal("swap onto GDB leaf should fail")
	}
	if a.focusedWidget() != gdb {
		t.Fatal("GDB leaf widget should be unchanged")
	}
	if len(a.widgetJump) != 0 {
		t.Fatal("refused swap must not push jump list")
	}
}

func TestSwapFocusedWidgetAllowsOtherLeaf(t *testing.T) {
	a := newGdbLeafApp()
	other := a.builtins["other"]
	codeLeaf := a.tab.LeafMark(leafMarkCode)
	if codeLeaf == nil {
		t.Fatal("missing code leaf mark")
	}
	if !a.tab.FocusLeaf(codeLeaf) {
		t.Fatal("focus code leaf")
	}
	if a.isGdbLeaf(a.focusedLeaf()) {
		t.Fatal("code leaf must not be GDB leaf")
	}
	if !a.swapFocusedWidget(other) {
		t.Fatal("swap onto non-GDB leaf should succeed")
	}
	if a.focusedWidget() != other {
		t.Fatal("expected other widget on focused leaf")
	}
	gdbLeaf := a.tab.LeafMark(leafMarkGDB)
	if gdbLeaf == nil || gdbLeaf.GetWidget() != a.gdbWidget {
		t.Fatal("GDB leaf must still show gdbWidget")
	}
}

func TestJumpBackRefusesGdbLeaf(t *testing.T) {
	a := newGdbLeafApp()
	other := a.builtins["other"]
	a.widgetJump = []termui.Widget{other}

	a.JumpBack()
	if a.focusedWidget() != a.gdbWidget {
		t.Fatal("JumpBack must not replace GDB leaf")
	}
	if len(a.widgetJump) != 1 || a.widgetJump[0] != other {
		t.Fatal("JumpBack refuse must leave jump stack untouched")
	}
}
