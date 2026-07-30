package debugstate

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

func TestDebugStateDefaultsAndSuppress(t *testing.T) {
	app := platform.NewAppState()
	s := New(app)
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
	s.SetGdbListenPrint(false)
	app.WithPTYOwner(platform.PTYOwnerMCP, func() {})
	if !s.SuppressGdbConsole() {
		t.Fatal("sticky silent after MCP write")
	}
	app.WithPTYOwner(platform.PTYOwnerUI, func() {})
	if s.SuppressGdbConsole() {
		t.Fatal("UI write clears sticky silent")
	}
	s.SetGdbListenPrint(true)
	if s.SuppressGdbConsole() {
		t.Fatal("gdblistenprint disables suppress")
	}
	if !s.ClearOutput() || !s.BreakMain() {
		t.Fatal("clearoutput/breakmain defaults")
	}
	s.SetSourceFiles([]string{"/a.c"})
	s.SetCurrentLocation("/a.c", 42)
	if s.CurrentFile() != "/a.c" || s.CurrentLine() != 42 {
		t.Fatal("location")
	}
	if s.BreakColor() != tcell.ColorRed {
		t.Fatal("breakcolor default")
	}
	if s.BreakCondColor() != platform.DefaultBreakCondColor {
		t.Fatal("breakcondcolor default")
	}
	if s.PCColor() != platform.DefaultPCColor {
		t.Fatal("pccolor default")
	}
	if s.ConsumeStopUISuppress() {
		t.Fatal("suppress should be empty")
	}
	s.NoteTransientStopSuppress()
	s.NoteTransientStopSuppress()
	if !s.ConsumeStopUISuppress() || !s.ConsumeStopUISuppress() || s.ConsumeStopUISuppress() {
		t.Fatal("suppress consume mismatch")
	}
}
