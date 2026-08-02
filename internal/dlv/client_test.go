package dlv

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

func TestClientIsSession(t *testing.T) {
	var _ core.Session = (*Client)(nil)
	var _ core.Session = (*ptyx.Client)(nil)
}

func TestNewClientRequiresProg(t *testing.T) {
	_, err := NewClient("dlv", nil)
	if err == nil {
		t.Fatal("expected error for empty args")
	}
}

func TestNewClientStartsAndCloses(t *testing.T) {
	if _, err := exec.LookPath("dlv"); err != nil {
		t.Skip("dlv not on PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	prog := filepath.Join(dir, "dlvhello")
	if err := os.WriteFile(src, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", prog, src)
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("go build: %v (%s)", err, out)
	}

	client, err := NewClient("dlv", []string{prog})
	if err != nil {
		t.Skipf("dlv/pty unavailable: %v", err)
	}
	defer client.Close()

	if boot := client.TakeStartupOutput(); boot == "" {
		t.Fatal("expected startup output through first (dlv) prompt")
	}

	// Ensure Send does not panic.
	_ = client.Send("help")
	time.Sleep(50 * time.Millisecond)
}
