package platform

import "testing"

func TestAppStatePTYOwnerAndEqualAlways(t *testing.T) {
	var s AppState
	if s.PTYOwner() != PTYOwnerNone {
		t.Fatal("default owner")
	}
	s.WithPTYOwner(PTYOwnerMCP, func() {
		if s.PTYOwner() != PTYOwnerMCP {
			t.Fatal("owner during WithPTYOwner")
		}
	})
	if s.PTYOwner() != PTYOwnerNone {
		t.Fatal("owner restored")
	}

	s.SetEqualAlways(true)
	if !s.EqualAlways() {
		t.Fatal("equalalways")
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
	_ = PTYOwnerApp.String()
}
