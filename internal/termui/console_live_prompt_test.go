package termui

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestConsolePaneLivePromptAttach(t *testing.T) {
	p := NewConsolePane("test")
	p.Prompt = "(gdb) "
	p.EnsureLivePrompt()
	if !p.LivePrompt() {
		t.Fatal("expected live prompt after EnsureLivePrompt")
	}
	if n := p.buf.NumLines(); n != 1 || p.buf.Line(0) != "(gdb) " {
		t.Fatalf("buf=%v", p.buf.Lines())
	}

	p.EchoSubmit("help")
	if p.LivePrompt() {
		t.Fatal("EchoSubmit should clear live prompt")
	}
	if p.buf.Line(0) != "(gdb) help" {
		t.Fatalf("line=%q", p.buf.Line(0))
	}

	p.EnsureLivePrompt()
	if p.buf.NumLines() != 2 || p.buf.Line(1) != "(gdb) " {
		t.Fatalf("buf=%v", p.buf.Lines())
	}

	p.AppendLines([]string{"~output"})
	if p.LivePrompt() {
		t.Fatal("AppendLines should drop live prompt host")
	}
	// live host removed, then output appended
	if got := p.buf.Lines(); len(got) < 2 || got[len(got)-1] != "~output" {
		t.Fatalf("buf=%v", got)
	}
}

func TestVisibleWidthAfterANSIPrompt(t *testing.T) {
	host := "\x1b[01;32muser\x1b[00m $ "
	if VisibleANSIWidth(host) != len("user $ ") {
		t.Fatalf("width=%d", VisibleANSIWidth(host))
	}
	_ = tcell.StyleDefault
}
