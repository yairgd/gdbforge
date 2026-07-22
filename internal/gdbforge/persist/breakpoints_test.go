package persist

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yairgd/gdbforge/internal/mcp"
)

func TestSaveLoadBreakpoints(t *testing.T) {
	dir := t.TempDir()
	items := []mcp.BreakInfo{
		{File: "/src/main.c", Line: 42, Enabled: true},
		{File: "hello.c", Line: 10, Enabled: false},
	}
	if err := SaveBreakpoints(dir, items); err != nil {
		t.Fatal(err)
	}
	path := BreakpointsPath(dir)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	got, err := LoadBreakpoints(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].File != "/src/main.c" || got[0].Line != 42 || !got[0].Enabled {
		t.Fatalf("first=%+v", got[0])
	}
	if got[1].File != "hello.c" || got[1].Enabled {
		t.Fatalf("second=%+v", got[1])
	}
}

func TestSaveBreakpointsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := SaveBreakpoints(dir, nil); err != nil {
		t.Fatal(err)
	}
	got, err := LoadBreakpoints(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got=%v", got)
	}
	raw, err := os.ReadFile(BreakpointsPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "breakpoints:") {
		t.Fatalf("raw=%q", raw)
	}
}

func TestLoadBreakpointsMissing(t *testing.T) {
	got, err := LoadBreakpoints(t.TempDir())
	if err != nil || got != nil {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestBreakpointsPath(t *testing.T) {
	want := filepath.Join("build", DirName, BreakpointsFile)
	if got := BreakpointsPath("build"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
