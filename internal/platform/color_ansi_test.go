package platform

import (
	"strings"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestColorANSI256RedYellow(t *testing.T) {
	if got := ColorANSI256(tcell.ColorRed); got != 196 {
		t.Fatalf("red=%d want 196", got)
	}
	if got := ColorANSI256(tcell.ColorYellow); got != 226 {
		t.Fatalf("yellow=%d want 226", got)
	}
}

func TestBreakNumberANSI(t *testing.T) {
	s := BreakNumberANSI("   2", tcell.ColorRed)
	if !strings.Contains(s, "48;5;196") || !strings.Contains(s, "38;5;231") {
		t.Fatalf("red gutter=%q", s)
	}
	s = BreakNumberANSI("   2", tcell.ColorYellow)
	if !strings.Contains(s, "48;5;226") || !strings.Contains(s, "38;5;16") {
		t.Fatalf("yellow gutter=%q", s)
	}
}
