package widgets

import (
	"strings"
	"testing"
)

func TestAboutWidgetCachesStaticContent(t *testing.T) {
	w := NewAboutWidget("v1.2.3")
	lines := w.LinesForTest()
	if len(lines) == 0 {
		t.Fatal("expected about lines")
	}
	if lines[0] != "gdbforge" {
		t.Fatalf("title: %q", lines[0])
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"Version:",
		"v1.2.3",
		"Yair Gadelov",
		"gdbforge: Extreme Tooling Suite",
		"https://github.com/yairgd/gdbforge",
		"MIT License",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in about text", want)
		}
	}
}

func TestAboutWidgetNotForRelease(t *testing.T) {
	w := NewAboutWidget("dev")
	joined := strings.Join(w.LinesForTest(), "\n")
	if !strings.Contains(joined, AboutNotForRelease) {
		t.Fatalf("want %q in about text:\n%s", AboutNotForRelease, joined)
	}
	if strings.Contains(joined, "\n    dev\n") {
		t.Fatal("raw dev stamp should not appear as version line")
	}
}

func TestFormatAboutVersion(t *testing.T) {
	cases := map[string]string{
		"":                       AboutNotForRelease,
		"dev":                    AboutNotForRelease,
		"v1.0.0":                 "v1.0.0",
		"1.0.0":                  "v1.0.0",
		"v1.0.0-rc.1":            "v1.0.0-rc.1",
		"v0.3.0-rc.1-9-g0713b6f": AboutNotForRelease,
	}
	for in, want := range cases {
		if got := FormatAboutVersion(in); got != want {
			t.Fatalf("FormatAboutVersion(%q)=%q want %q", in, got, want)
		}
	}
}

func TestFormatBuildLineUnknown(t *testing.T) {
	if got := FormatBuildLine("Git SHA", ""); got != "    Git SHA: unknown" {
		t.Fatalf("got %q", got)
	}
}
