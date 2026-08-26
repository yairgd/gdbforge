package main

import (
	"testing"

	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/termui"
)

// newLocationLeafApp builds Code | GDB with a real CodeWidget + AssemblyWidget
// so presentLocation / leaf-mark reclaim can be exercised without a GDB session.
func newLocationLeafApp() *DebuggerApp {
	code := widgets.NewCodeWidget()
	asm := widgets.NewAssemblyWidget()
	gdb := widgets.NewGDBWidget()
	tab := termui.NewTabTwoHozSplitWins("test", code, gdb)
	a := &DebuggerApp{
		DebugSession: DebugSession{gdbWidget: gdb},
	}
	initLayoutShell(a, tab)
	a.asm.host = a
	a.asm.widget = asm
	a.bufs.host = a
	a.bufs.initMaps()
	a.bufs.setPrimary(code)
	codeLeaf := a.Tab().FindLeaf(func(w termui.Widget) bool { return w == code })
	a.Tab().SetLeafMark(leafMarkCode, codeLeaf)
	a.Tab().SetLeafMark(leafMarkGDB, a.Tab().FindLeaf(func(w termui.Widget) bool { return w == gdb }))
	_ = a.Tab().FocusLeaf(codeLeaf)
	return a
}

func TestHasSplitIgnoresSharedLocationAsm(t *testing.T) {
	a := newLocationLeafApp()
	codeLeaf := a.Tab().LeafMark(leafMarkCode)
	codeLeaf.SetWidget(a.asm.Widget())
	// Contaminated mark: shared leaf also bookmarked as asm (old rememberCodeLeafFromFocus).
	a.Tab().SetLeafMark(leafMarkAsm, codeLeaf)
	if a.asm.hasSplit() {
		t.Fatal("shared location leaf must not count as :vs/:sp asm split")
	}
}

func TestRememberFocusKeepsSharedAsmOnCodeMark(t *testing.T) {
	a := newLocationLeafApp()
	codeLeaf := a.Tab().LeafMark(leafMarkCode)
	codeLeaf.SetWidget(a.asm.Widget())
	_ = a.Tab().FocusLeaf(codeLeaf)
	a.rememberCodeLeafFromFocus()
	if a.Tab().LeafMark(leafMarkAsm) != nil {
		t.Fatal("shared Asm must not set leafMarkAsm")
	}
	if a.Tab().LeafMark(leafMarkCode) != codeLeaf {
		t.Fatal("shared Asm must keep leafMarkCode")
	}
}

func TestPresentLocationReclaimsCodeAfterAutoAsm(t *testing.T) {
	a := newLocationLeafApp()
	code := a.bufs.Primary()
	aw := a.asm.Widget()
	codeLeaf := a.Tab().LeafMark(leafMarkCode)

	// Simulate autoAsm: location leaf shows Assembly, focus still on that leaf.
	a.asm.setAutoAsm(true)
	codeLeaf.SetWidget(aw)
	_ = a.Tab().FocusLeaf(codeLeaf)
	a.rememberCodeLeafFromFocus()
	if a.asm.hasSplit() {
		t.Fatal("autoAsm must not look like a dedicated split")
	}

	// Source returns — Code should reclaim the shared leaf.
	a.presentLocation(code, nil)
	if a.asm.AutoAsm() {
		t.Fatal("autoAsm should clear when source is available")
	}
	if codeLeaf.GetWidget() != code {
		t.Fatalf("expected CodeWidget back in location leaf, got %T", codeLeaf.GetWidget())
	}
}

func TestPresentLocationReclaimsCodeWhenFocusElsewhere(t *testing.T) {
	a := newLocationLeafApp()
	code := a.bufs.Primary()
	aw := a.asm.Widget()
	codeLeaf := a.Tab().LeafMark(leafMarkCode)
	gdbLeaf := a.Tab().LeafMark(leafMarkGDB)

	a.asm.setAutoAsm(true)
	codeLeaf.SetWidget(aw)
	// Old bug: mark shared leaf as asm, then focus GDB so reclaim uses findCodeLeaf.
	a.Tab().SetLeafMark(leafMarkAsm, codeLeaf)
	_ = a.Tab().FocusLeaf(gdbLeaf)

	a.presentLocation(code, nil)
	if codeLeaf.GetWidget() != code {
		t.Fatalf("expected Code reclaim with GDB focused, got %T", codeLeaf.GetWidget())
	}
	if a.Tab().LeafMark(leafMarkAsm) != nil {
		t.Fatal("mistaken asm mark on shared leaf should be cleared")
	}
}
