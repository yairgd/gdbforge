package widgets

import (
	"strings"
	"testing"
)

func TestHelpWidgetContent(t *testing.T) {
	w := NewHelpWidget()
	lines := w.LinesForTest()
	if len(lines) == 0 {
		t.Fatal("expected help lines")
	}
	if lines[0] != "gdbforge — user manual" {
		t.Fatalf("title: %q", lines[0])
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		":help",
		"=== Modes ===",
		"=== Colon commands ===",
		"=== Per-pane reference ===",
		"Space",
		":b about",
		"Viewport",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in help text", want)
		}
	}
	if strings.Contains(joined, "Yair Gadelov") {
		t.Fatal("help must not include author credit (use :b about)")
	}
}
