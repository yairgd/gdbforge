package luahost

import (
	"strings"
	"testing"
)

func TestAPIHelpAll(t *testing.T) {
	lines := APIHelp("")
	if len(lines) == 0 {
		t.Fatal("expected help lines")
	}
	text := strings.Join(lines, "\n")
	for _, want := range []string{"gdbforge.gdb(", "gdbforge.print(", "pane.set_cell", "Example:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in help", want)
		}
	}
}

func TestAPIHelpTopic(t *testing.T) {
	lines := APIHelp("gdb")
	if len(lines) == 0 {
		t.Fatal("expected gdb entries")
	}
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, "gdbforge.gdb(") {
		t.Fatalf("missing gdb entry: %q", text)
	}
	if strings.Contains(text, "gdbforge.spawn(") {
		t.Fatalf("spawn should not match gdb topic")
	}
}

func TestAPIHelpNoMatch(t *testing.T) {
	lines := APIHelp("zzznomatch")
	if len(lines) != 1 || !strings.Contains(lines[0], "no help entries") {
		t.Fatalf("got %v", lines)
	}
}

func TestGdbforgeHelpFunc(t *testing.T) {
	var lines []string
	rt := New(nil, nil)
	rt.SetPrintSink(func(line string) { lines = append(lines, line) })
	if err := rt.EvalLine(`gdbforge.help("print")`); err != nil {
		t.Fatal(err)
	}
	if len(lines) == 0 {
		t.Fatal("expected output")
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "gdbforge.print") {
		t.Fatalf("got %q", joined)
	}
}

func TestGlobalHelpInREPL(t *testing.T) {
	var lines []string
	rt := New(nil, nil)
	rt.SetPrintSink(func(line string) { lines = append(lines, line) })
	if err := rt.LoadString(`help = function(...) gdbforge.help(...) end`, "@repl"); err != nil {
		t.Fatal(err)
	}
	if err := rt.EvalLine(`help("spawn")`); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "gdbforge.spawn") {
		t.Fatalf("got %q", joined)
	}
}

func TestEvalLineBareHelp(t *testing.T) {
	var lines []string
	rt := New(nil, nil)
	rt.SetPrintSink(func(line string) { lines = append(lines, line) })
	if err := rt.LoadString(`help = function(...) gdbforge.help(...) end`, "@repl"); err != nil {
		t.Fatal(err)
	}
	lines = nil
	if err := rt.EvalLine("help"); err != nil {
		t.Fatalf("bare help: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("expected help output")
	}
	lines = nil
	if err := rt.EvalLine("help gdb"); err != nil {
		t.Fatalf("help gdb: %v", err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "gdbforge.gdb") {
		t.Fatalf("got %q", joined)
	}
}
