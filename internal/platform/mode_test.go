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
	var hooked PTYOwner = PTYOwnerNone
	s.SetPTYOwnerHook(func(o PTYOwner) { hooked = o })
	s.WithPTYOwner(PTYOwnerMCP, func() {
		if s.PTYOwner() != PTYOwnerMCP {
			t.Fatal("owner during WithPTYOwner")
		}
		if hooked != PTYOwnerMCP {
			t.Fatal("hook during WithPTYOwner")
		}
	})
	if s.PTYOwner() != PTYOwnerNone {
		t.Fatal("owner restored")
	}
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
	if s.CodeSelColor() != DefaultCodeSelColor {
		t.Fatal("codeselcolor default")
	}
	if s.MutedColor() != DefaultMutedColor {
		t.Fatal("mutedcolor default")
	}
	if s.SearchColor() != DefaultSearchColor {
		t.Fatal("searchcolor default darkorange")
	}
	s.SetSearchColor(tcell.ColorAqua)
	if s.SearchColor() != tcell.ColorAqua {
		t.Fatal("searchcolor set")
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
	if got := CompleteColorName("dark"); len(got) == 0 {
		t.Fatal("CompleteColorName dark")
	}
	for _, name := range CompleteColorName("") {
		if _, ok := ParseColorName(name); !ok {
			t.Fatalf("CompleteColorName %q not parseable", name)
		}
	}
	_ = PTYOwnerApp.String()
}

func TestAppStateLayoutsAndEscToCode(t *testing.T) {
	s := NewAppState()
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
