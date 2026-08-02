package widgets

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/mitext"
	"strings"
	"testing"

	"github.com/yairgd/gdbforge/internal/termui"
)

func testGDBWidget() *GDBWidget {
	return NewGDBWidget()
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
	w.EchoSubmit("continue")
	if w.LivePrompt() {
		t.Fatal("waiting: livePrompt should be false")
	}
	for _, line := range w.console.Buffer().Lines() {
		if strings.TrimSpace(line) == mitext.MIPromptToken || strings.HasPrefix(line, mitext.MIPromptToken) {
			t.Fatalf("invented prompt while waiting: %v", w.console.Buffer().Lines())
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
		if strings.Contains(b.String(), mitext.MIPromptToken) {
			t.Fatalf("Draw paints fake prompt on row %d: %q", y, strings.TrimRight(b.String(), " "))
		}
	}
}

func TestGDBWidgetPaintMiDisplayAttachesPrompt(t *testing.T) {
	w := testGDBWidget()
	w.EchoSubmit("help")
	w.PaintMiDisplay(MiPaintUpdate{
		DisplayLines: []string{"List of classes of commands:"},
		PromptReady:  true,
		PromptLine:   mitext.MIPromptToken,
	}, false, false)
	if !w.LivePrompt() {
		t.Fatal("PromptReady should set live prompt")
	}
	if got := bufLast(w); got != mitext.MIPromptLiveHost {
		t.Fatalf("last line=%q want %q", got, mitext.MIPromptLiveHost)
	}
}

func TestGDBWidgetPaintMiDisplayWithoutPromptLineDoesNotInvent(t *testing.T) {
	w := testGDBWidget()
	w.PaintMiDisplay(MiPaintUpdate{PromptReady: true}, false, false)
	if w.LivePrompt() {
		t.Fatal("PromptReady without PromptLine must not invent a host")
	}
	for _, line := range w.console.Buffer().Lines() {
		if strings.Contains(line, mitext.MIPromptToken) {
			t.Fatalf("invented prompt: %v", w.console.Buffer().Lines())
		}
	}
}

func TestGDBWidgetBeginLiveHost(t *testing.T) {
	w := testGDBWidget()
	w.console.Buffer().AppendLine(mitext.MIPromptToken)
	w.SetLivePrompt(true)

	w.BeginLiveHost(QuitConfirmLines("1234"), QuitConfirmHost)
	if strings.TrimSpace(bufLast(w)) != strings.TrimSpace(QuitConfirmHost) {
		t.Fatalf("quit host=%q", bufLast(w))
	}

	// After cancel, view does not invent (gdb); controller waits for MI PromptReady.
	w.SetLivePrompt(false)
	w.PaintMiDisplay(MiPaintUpdate{PromptReady: true, PromptLine: mitext.MIPromptToken}, false, false)
	if !w.LivePrompt() {
		t.Fatal("PromptReady after quit n should attach host")
	}
	if got := bufLast(w); got != mitext.MIPromptLiveHost {
		t.Fatalf("last=%q want %q", got, mitext.MIPromptLiveHost)
	}
}
