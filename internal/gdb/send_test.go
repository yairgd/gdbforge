package gdb

import "testing"

func TestIsContinueCmd(t *testing.T) {
	yes := []string{"c", "continue", "  continue  ", "c\n"}
	no := []string{"", "n", "next", "s", "step", "run", "target remote /dev/pts/1"}
	for _, cmd := range yes {
		if !IsContinueCmd(cmd) {
			t.Fatalf("IsContinueCmd(%q) = false, want true", cmd)
		}
	}
	for _, cmd := range no {
		if IsContinueCmd(cmd) {
			t.Fatalf("IsContinueCmd(%q) = true, want false", cmd)
		}
	}
}
