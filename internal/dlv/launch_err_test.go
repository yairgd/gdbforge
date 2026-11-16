package dlv

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

// A dlv that dies at launch must fail fast and carry dlv's own message.
func TestHeadlessLaunchFailsFastWithDlvOutput(t *testing.T) {
	if _, err := exec.LookPath("dlv"); err != nil {
		t.Skip("dlv not installed")
	}
	start := time.Now()
	_, err := NewClient("dlv", []string{"./no-such-program"})
	if err == nil || !strings.Contains(err.Error(), "dlv: could not launch") {
		t.Fatalf("want dlv output in error, got %v", err)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("took %v, want fast failure", d)
	}
}
