package termui

import (
	"strings"
	"testing"

	"github.com/yairgd/gdbforge/internal/gdb"
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
		if strings.Contains(row, gdb.MIPromptToken) {
			t.Fatalf("row %d paints fake prompt: %q", y, strings.TrimRight(row, " "))
		}
	}
}

func TestConsolePaneEchoSubmitOntoLiveHost(t *testing.T) {
	p := NewConsolePane("gdb")
	p.Prompt = ""
	p.buf.AppendLine(gdb.MIPromptLiveHost)
	p.SetLivePrompt(true)
	p.EchoSubmit("help")
	if p.LivePrompt() {
		t.Fatal("EchoSubmit should clear live prompt")
	}
	if got := p.buf.Line(0); got != gdb.MIPromptLiveHost+"help" {
		t.Fatalf("line=%q", got)
	}
}

func TestConsolePaneStripTrailingBarePromptEmptyIgnoresMIToken(t *testing.T) {
	// Empty Prompt: ConsolePane does not hardcode MI tokens; GDBWidget strips those.
	p := NewConsolePane("gdb")
	p.Prompt = ""
	p.buf.AppendLine("output")
	p.buf.AppendLine(gdb.MIPromptLiveHost)
	p.SetLivePrompt(true)
	p.StripTrailingBarePrompt()
	if n := p.buf.NumLines(); n != 2 {
		t.Fatalf("expected MI host left in place, buf=%v", p.buf.Lines())
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

func TestConsolePaneLivePromptAttachedOnLastViewportRow(t *testing.T) {
	p := NewConsolePane("gdb")
	p.Prompt = ""
	const w, h = 40, 6
	for i := 0; i < h+3; i++ {
		p.buf.AppendLine("line")
	}
	p.buf.AppendLine(gdb.MIPromptLiveHost)
	p.SetLivePrompt(true)
	p.Input().InsertText("help")

	g := NewGrid(w, h)
	c := NewCanvas(g).WithRect(NewRect(0, 0, w, h))
	p.Draw(c)

	want := strings.TrimRight(gdb.MIPromptLiveHost+"help", " ")
	bottom := strings.TrimRight(gridRow(g, h-1, w), " ")
	if bottom != want {
		t.Fatalf("bottom row=%q want %q", bottom, want)
	}
	above := strings.TrimRight(gridRow(g, h-2, w), " ")
	if strings.Contains(above, gdb.MIPromptToken) {
		t.Fatalf("prompt duplicated above bottom: %q", above)
	}
}
