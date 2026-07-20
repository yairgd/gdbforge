package widgets

import (
	"strings"
	"testing"
)

func TestLogoLines(t *testing.T) {
	lines := LogoLinesForTest()
	if len(lines) < 7 {
		t.Fatalf("too few logo lines: %d", len(lines))
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Extreme GDB Tooling Suite") {
		t.Fatal("missing tagline")
	}
	if !strings.Contains(joined, "██") {
		t.Fatal("missing ASCII block logo")
	}
}
