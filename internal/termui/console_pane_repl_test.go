package termui

import "testing"

func TestConsolePaneAppendScrollbackLineDoesNotGluePrompt(t *testing.T) {
	p := NewConsolePane("Lua")
	p.Prompt = "lua> "
	p.EnsureLivePrompt()
	p.input.SetText("print(a)")
	p.EchoSubmit("print(a)")
	p.input.Clear()
	p.EnsureLivePrompt()

	p.AppendScrollbackLine("1")

	if p.LivePrompt() {
		t.Fatal("live prompt should be off after scrollback append")
	}
	lines := p.buf.Lines()
	if len(lines) != 2 {
		t.Fatalf("lines=%v", lines)
	}
	if lines[0] != "lua> print(a)" {
		t.Fatalf("command line: %q", lines[0])
	}
	if lines[1] != "1" {
		t.Fatalf("output glued or wrong: %q", lines[1])
	}
}
