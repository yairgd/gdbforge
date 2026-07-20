package widgets

import (
	"strings"
	"testing"
)

func TestAboutWidgetCachesStaticContent(t *testing.T) {
	w := NewAboutWidget()
	lines := w.LinesForTest()
	if len(lines) == 0 {
		t.Fatal("expected about lines")
	}
	if lines[0] != "xgdb" {
		t.Fatalf("title: %q", lines[0])
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"Version:",
		"0.1.0",
		"Yair Gadelov",
		"Extreme GDB Tooling Suite",
		"https://github.com/yairgd/newcgdb",
		"MIT License",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in about text", want)
		}
	}
}

func TestFormatBuildLineUnknown(t *testing.T) {
	if got := FormatBuildLine("Git SHA", ""); got != "    Git SHA: unknown" {
		t.Fatalf("got %q", got)
	}
}
