package termui

import "testing"

func TestPaintInactiveStatusBarStartsAtFourthChar(t *testing.T) {
	if inactiveNameCol != 3 {
		t.Fatalf("inactiveNameCol=%d want 3 (4th character)", inactiveNameCol)
	}
}
