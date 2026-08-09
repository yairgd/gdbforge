package platform

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestContrastColor(t *testing.T) {
	if got := ContrastColor(tcell.ColorYellow); got != tcell.ColorBlack {
		t.Fatalf("yellow bg want black fg, got %v", got)
	}
	if got := ContrastColor(tcell.ColorRed); got != tcell.ColorWhite {
		t.Fatalf("red bg want white fg, got %v", got)
	}
}
