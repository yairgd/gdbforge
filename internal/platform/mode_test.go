package platform

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestAppStatePTYOwnerAndEqualAlways(t *testing.T) {
	s := NewAppState()
	if s.PTYOwner() != PTYOwnerNone {
		t.Fatal("default owner")
	}
	if !s.EqualAlways() {
		t.Fatal("equalalways default true")
	}
	r := s.DefaultLayoutRatios()
	if r.Left < 0.66 || r.Left > 0.67 {
		t.Fatalf("Left default=%v want ~2/3", r.Left)
	}
	if r.Output != 0.5 {
		t.Fatalf("Output default=%v want 1/2", r.Output)
	}
	if r.BottomFirst < 0.33 || r.BottomFirst > 0.34 {
		t.Fatalf("BottomFirst default=%v want ~1/3", r.BottomFirst)
	}
	if s.LayoutLeftRatio() != r.Left {
		t.Fatal("LayoutLeftRatio mirrors DefaultLayoutRatios.Left")
	}
	s.WithPTYOwner(PTYOwnerMCP, func() {
		if s.PTYOwner() != PTYOwnerMCP {
			t.Fatal("owner during WithPTYOwner")
		}
	})
	if s.PTYOwner() != PTYOwnerNone {
		t.Fatal("owner restored")
	}
	if !s.GdbListenPrint() {
		t.Fatal("gdblistenprint default true")
	}
	if s.GdbTargetPrint() {
		t.Fatal("gdbtargetprint default false")
	}
	s.SetGdbTargetPrint(true)
	if !s.GdbTargetPrint() {
		t.Fatal("gdbtargetprint")
	}
	s.SetGdbTargetPrint(false)
	if s.GdbTargetPrint() {
		t.Fatal("nogdbtargetprint")
	}
	// Sticky silence only suppresses when listen-print is off.
	s.SetGdbListenPrint(false)
	if !s.SuppressGdbConsole() {
		t.Fatal("sticky silent after MCP write")
	}
	s.WithPTYOwner(PTYOwnerUI, func() {})
	if s.SuppressGdbConsole() {
		t.Fatal("UI write clears sticky silent")
	}
	s.WithPTYOwner(PTYOwnerApp, func() {
		if !s.SuppressGdbConsole() {
			t.Fatal("suppress during App owner")
		}
	})
	if !s.SuppressGdbConsole() {
		t.Fatal("sticky silent after App write")
	}
	s.SetGdbListenPrint(true)
	if s.SuppressGdbConsole() {
		t.Fatal("gdblistenprint disables suppress")
	}
	s.SetGdbListenPrint(false)

	s.SetEqualAlways(false)
	if s.EqualAlways() {
		t.Fatal("noequalalways")
	}
	s.SetEqualAlways(true)
	if !s.EqualAlways() {
		t.Fatal("equalalways")
	}
	s.SetLayoutLeftRatio(0.5)
	if s.LayoutLeftRatio() != 0.5 {
		t.Fatal("layoutLeftRatio set")
	}
	s.SetLayoutLeftRatio(0.01)
	if s.LayoutLeftRatio() != 0.1 {
		t.Fatal("layoutLeftRatio clamp low")
	}
	s.SetLayoutLeftRatio(0.99)
	if s.LayoutLeftRatio() != 0.9 {
		t.Fatal("layoutLeftRatio clamp high")
	}
	s.SetMode(ModeCommand)
	if s.Mode() != ModeCommand {
		t.Fatal("mode")
	}

	s.SetSourceFiles([]string{"/a.c", "/b.c"})
	if len(s.SourceFiles()) != 2 {
		t.Fatal("source files")
	}
	s.SetCurrentLocation("/a.c", 42)
	if s.CurrentFile() != "/a.c" || s.CurrentLine() != 42 {
		t.Fatal("location")
	}
	if s.MarkColor() != tcell.ColorBlue {
		t.Fatal("markcolor default blue")
	}
	s.SetMarkColor(tcell.ColorNavy)
	if s.MarkColor() != tcell.ColorNavy {
		t.Fatal("markcolor set")
	}
	if s.MarkDimColor() != tcell.ColorGray {
		t.Fatal("markdimcolor default gray")
	}
	s.SetMarkDimColor(tcell.ColorSilver)
	if s.MarkDimColor() != tcell.ColorSilver {
		t.Fatal("markdimcolor set")
	}
	if s.BreakColor() != tcell.ColorRed {
		t.Fatal("breakcolor default red")
	}
	if s.BreakDisabledColor() != tcell.ColorYellow {
		t.Fatal("breakdisabledcolor default yellow")
	}
	if s.PCColor() != DefaultPCColor {
		t.Fatal("pccolor default")
	}
	if s.StackBreakColor() != DefaultStackBreakColor {
		t.Fatal("stackbreakcolor default")
	}
	if s.CodeSelColor() != DefaultCodeSelColor {
		t.Fatal("codeselcolor default")
	}
	if s.MutedColor() != DefaultMutedColor {
		t.Fatal("mutedcolor default")
	}
	s.SetBreakColor(tcell.ColorPurple)
	s.SetBreakDisabledColor(tcell.ColorGray)
	if s.BreakColor() != tcell.ColorPurple || s.BreakDisabledColor() != tcell.ColorGray {
		t.Fatal("break colors set")
	}
	s.SetPCColor(tcell.ColorNavy)
	if s.PCColor() != tcell.ColorNavy {
		t.Fatal("pccolor set")
	}
	if c, ok := ParseColorName("darkblue"); !ok || c != tcell.ColorDarkBlue {
		t.Fatal("ParseColorName darkblue")
	}
	if c, ok := ParseColorName("darkslategray"); !ok || c != tcell.ColorDarkSlateGray {
		t.Fatal("ParseColorName darkslategray")
	}
	if _, ok := ParseColorName("notaColor"); ok {
		t.Fatal("ParseColorName unknown")
	}
	_ = PTYOwnerApp.String()
}

func TestAppStateClearOutputAndLayouts(t *testing.T) {
	s := NewAppState()
	if !s.ClearOutput() {
		t.Fatal("clearoutput default true")
	}
	s.SetClearOutput(false)
	if s.ClearOutput() {
		t.Fatal("noclearoutput")
	}
	if s.ContinueAfterClear() {
		t.Fatal("continueafterclear default false")
	}
	s.SetContinueAfterClear(true)
	if !s.ContinueAfterClear() {
		t.Fatal("continueafterclear on")
	}
	s.SetContinueAfterClear(false)
	if s.ContinueAfterClear() {
		t.Fatal("nocontinueafterclear")
	}
	if !s.EscToCode() {
		t.Fatal("esctocode default true")
	}
	s.SetEscToCode(false)
	if s.EscToCode() {
		t.Fatal("noesctocode")
	}
	s.SetEscToCode(true)
	if !s.EscToCode() {
		t.Fatal("esctocode on")
	}
	if !s.BreakMain() {
		t.Fatal("breakmain default true")
	}
	s.SetBreakMain(false)
	if s.BreakMain() {
		t.Fatal("nobreakmain")
	}
	s.SetBreakMain(true)
	if !s.BreakMain() {
		t.Fatal("breakmain on")
	}
	if !s.HasLayout(LayoutDefault) || s.CurrentLayout() != LayoutDefault {
		t.Fatal("default layout")
	}
	s.RegisterLayout("wide")
	if !s.HasLayout("wide") {
		t.Fatal("register layout")
	}
	s.RegisterLayout("wide") // idempotent
	if len(s.Layouts()) != 2 {
		t.Fatalf("layouts=%v", s.Layouts())
	}
	s.SetCurrentLayout("wide")
	if s.CurrentLayout() != "wide" {
		t.Fatal("current layout")
	}
}
