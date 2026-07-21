package widgets

import (
	"strings"
	"testing"

	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/termui"
)

func testGDBWidget() *GDBWidget {
	console := termui.NewConsolePane("GDB")
	console.Prompt = ""
	console.LineStyle = gdbLineStyle
	return &GDBWidget{
		console:       console,
		gdbInputState: *gdb.NewGdbInputState(),
	}
}

func bufLast(w *GDBWidget) string {
	buf := w.console.Buffer()
	if buf == nil || buf.NumLines() == 0 {
		return ""
	}
	return buf.Line(buf.NumLines() - 1)
}

func TestGDBWidgetNoFakePromptWhileWaiting(t *testing.T) {
	w := testGDBWidget()
	w.console.Buffer().AppendLine("Breakpoint 1 at 0x100")
	w.console.EchoSubmit("continue")
	if w.console.LivePrompt() {
		t.Fatal("waiting: livePrompt should be false")
	}
	for _, line := range w.console.Buffer().Lines() {
		if strings.TrimSpace(line) == "(gdb)" || strings.HasPrefix(line, "(gdb)") {
			t.Fatalf("invented (gdb) while waiting: %v", w.console.Buffer().Lines())
		}
	}

	const width, height = 48, 8
	g := termui.NewGrid(width, height)
	c := termui.NewCanvas(g).WithRect(termui.NewRect(0, 0, width, height))
	w.Draw(c)
	for y := 0; y < height; y++ {
		var b strings.Builder
		for x := 0; x < width; x++ {
			ch := g.Cells[x][y].Rune
			if ch == 0 {
				ch = ' '
			}
			b.WriteRune(ch)
		}
		if strings.Contains(b.String(), "(gdb)") {
			t.Fatalf("Draw paints fake (gdb) on row %d: %q", y, strings.TrimRight(b.String(), " "))
		}
	}
}

func TestGDBWidgetPromptReadyAttachesHost(t *testing.T) {
	w := testGDBWidget()
	w.console.EchoSubmit("help")
	w.applyMiUpdate(gdb.MiUpdate{
		DisplayLines: []string{"List of classes of commands:"},
		PromptReady:  true,
	})
	if !w.console.LivePrompt() {
		t.Fatal("PromptReady should set live prompt")
	}
	if got := bufLast(w); got != "(gdb) " {
		t.Fatalf("last line=%q want %q", got, "(gdb) ")
	}
}

func TestGDBWidgetQuitCancelWaitsForGDBPrompt(t *testing.T) {
	w := testGDBWidget()
	w.inferiorAlive = true
	w.inferiorPID = "1234"
	// Pretend we already have a live (gdb) host the user typed q on.
	w.console.Buffer().AppendLine("(gdb) ")
	w.console.SetLivePrompt(true)

	w.beginQuitConfirm("q")
	if !w.quitConfirm {
		t.Fatal("expected quitConfirm")
	}
	if strings.TrimSpace(bufLast(w)) != "Quit anyway? (y or n)" {
		t.Fatalf("quit host=%q", bufLast(w))
	}

	w.finishQuitConfirm("n")
	if w.quitConfirm {
		t.Fatal("quitConfirm should clear on n")
	}
	if w.console.LivePrompt() {
		t.Fatal("after n: no live prompt until GDB emits (gdb)")
	}
	for _, line := range w.console.Buffer().Lines() {
		if strings.TrimSpace(line) == "(gdb)" {
			t.Fatalf("invented (gdb) after quit n: %v", w.console.Buffer().Lines())
		}
	}

	w.applyMiUpdate(gdb.MiUpdate{PromptReady: true})
	if !w.console.LivePrompt() {
		t.Fatal("PromptReady after quit n should attach host")
	}
	if got := bufLast(w); got != "(gdb) " {
		t.Fatalf("last=%q", got)
	}
}
