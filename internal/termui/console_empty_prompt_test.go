package termui

import (
	"strings"
	"testing"
)

func gridRow(g *Grid, y, w int) string {
	var b strings.Builder
	for x := 0; x < w; x++ {
		ch := g.Cells[x][y].Rune
		if ch == 0 {
			ch = ' '
		}
		b.WriteRune(ch)
	}
	return b.String()
}

func TestConsolePaneEmptyPromptDrawHasNoFakeGdb(t *testing.T) {
	p := NewConsolePane("gdb")
	p.Prompt = ""
	p.buf.AppendLine("some output")
	// After submit: waiting for GDB — no live host, empty Prompt.
	p.EchoSubmit("help")
	if p.LivePrompt() {
		t.Fatal("expected livePrompt false after EchoSubmit")
	}

	const w, h = 40, 6
	g := NewGrid(w, h)
	c := NewCanvas(g).WithRect(NewRect(0, 0, w, h))
	p.Draw(c)

	for y := 0; y < h; y++ {
		row := gridRow(g, y, w)
		if strings.Contains(row, "(gdb)") {
			t.Fatalf("row %d paints fake (gdb): %q", y, strings.TrimRight(row, " "))
		}
	}
}

func TestConsolePaneEchoSubmitOntoBareGdbHost(t *testing.T) {
	p := NewConsolePane("gdb")
	p.Prompt = ""
	p.buf.AppendLine("(gdb) ")
	p.SetLivePrompt(true)
	p.EchoSubmit("help")
	if p.LivePrompt() {
		t.Fatal("EchoSubmit should clear live prompt")
	}
	if got := p.buf.Line(0); got != "(gdb) help" {
		t.Fatalf("line=%q", got)
	}
}

func TestConsolePaneStripTrailingBareGdbWithEmptyPrompt(t *testing.T) {
	p := NewConsolePane("gdb")
	p.Prompt = ""
	p.buf.AppendLine("output")
	p.buf.AppendLine("(gdb) ")
	p.SetLivePrompt(true)
	p.StripTrailingBarePrompt()
	if p.LivePrompt() {
		t.Fatal("strip should clear live prompt")
	}
	if n := p.buf.NumLines(); n != 1 || p.buf.Line(0) != "output" {
		t.Fatalf("buf=%v", p.buf.Lines())
	}
}

func TestConsolePaneEnsureLivePromptNoopWhenEmpty(t *testing.T) {
	p := NewConsolePane("gdb")
	p.Prompt = ""
	p.EnsureLivePrompt()
	if p.LivePrompt() || p.buf.NumLines() != 0 {
		t.Fatalf("EnsureLivePrompt with empty Prompt must be no-op: live=%v lines=%v",
			p.LivePrompt(), p.buf.Lines())
	}
}

// When scrollback fills the pane, the live (gdb) host must share the bottom
// row with the caret — not sit one line above an empty input row.
func TestConsolePaneLivePromptAttachedOnLastViewportRow(t *testing.T) {
	p := NewConsolePane("gdb")
	p.Prompt = ""
	const w, h = 40, 6
	for i := 0; i < h+3; i++ {
		p.buf.AppendLine("line")
	}
	p.buf.AppendLine("(gdb) ")
	p.SetLivePrompt(true)
	p.Input().InsertText("help")

	g := NewGrid(w, h)
	c := NewCanvas(g).WithRect(NewRect(0, 0, w, h))
	p.Draw(c)

	bottom := strings.TrimRight(gridRow(g, h-1, w), " ")
	if bottom != "(gdb) help" {
		t.Fatalf("bottom row=%q want %q", bottom, "(gdb) help")
	}
	above := strings.TrimRight(gridRow(g, h-2, w), " ")
	if strings.Contains(above, "(gdb)") {
		t.Fatalf("prompt duplicated above bottom: %q", above)
	}
}
