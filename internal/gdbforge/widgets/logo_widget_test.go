package widgets

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLogoLines(t *testing.T) {
	lines := LogoLinesForTest()
	if len(lines) < 7 {
		t.Fatalf("too few logo lines: %d", len(lines))
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, ">> gdbforge: Extreme Tooling Suite <<") {
		t.Fatal("missing tagline")
	}
	if !strings.Contains(joined, "██") {
		t.Fatal("missing ASCII block logo")
	}
	// The G, D and B prepended to FORGE.
	for _, glyph := range []string{"██║  ███╗", "██████╔╝██████╔╝██║"} {
		if !strings.Contains(joined, glyph) {
			t.Fatalf("missing GDB block glyph %q", glyph)
		}
	}
}

func TestBannerFitsPaneWidth(t *testing.T) {
	for _, width := range []int{200, 67, 66, 24, 23, 8} {
		lines := LogoLinesForWidthForTest(width)
		if len(lines) == 0 {
			t.Fatalf("width %d: empty banner", width)
		}
		for _, line := range lines {
			if n := utf8.RuneCountInString(line); n > width {
				t.Fatalf("width %d: line of %d runes overflows: %q", width, n, line)
			}
		}
	}
}

func TestBannerFloorIsPlainName(t *testing.T) {
	// Below the smallest variant there is nothing left to shrink; Draw clips.
	lines := LogoLinesForWidthForTest(3)
	if len(lines) != 1 || lines[0] != "gdbforge" {
		t.Fatalf("expected plain name fallback, got %q", lines)
	}
}
