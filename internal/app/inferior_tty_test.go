package app

import (
	"os"
	"strings"
	"testing"

	"github.com/yairgd/gdbforge/internal/ttyhold"
)

func TestInferiorTTYHoldShellExecsSelf(t *testing.T) {
	got := inferiorTTYHoldShell("/tmp/p a t h", "/tmp/pid")
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("no executable path: %v", err)
	}
	// `exec` keeps the terminal's child pid, which the helper needs to be the
	// session leader when it releases the pts.
	if !strings.HasPrefix(got, "exec "+shellSingleQuote(exe)+" "+ttyhold.Flag+" ") {
		t.Fatalf("hold shell = %q", got)
	}
	if !strings.Contains(got, shellSingleQuote("/tmp/p a t h")) {
		t.Fatalf("path file not quoted in %q", got)
	}
}
