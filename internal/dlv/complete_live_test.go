package dlv

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCompleteLocspecLive exercises "b main." + Tab against a real Delve
// session. Under the ConsolePane-era widget the `funcs` query was safe because
// gdbforge owned the input line; the xterm pane leaves the half-typed line
// inside Delve's line editor, so candidates must come from rpc2 instead.
func TestCompleteLocspecLive(t *testing.T) {
	if _, err := exec.LookPath("dlv"); err != nil {
		t.Skip("dlv not on PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	prog := filepath.Join(dir, "dlvcomplete")
	// noinline keeps helper in the binary's function table.
	code := "package main\n\n//go:noinline\nfunc helper() int { return 1 }\n\nfunc main() { _ = helper() }\n"
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
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

	res := Complete(client, nil, "b main.")
	if len(res.Matches) == 0 {
		t.Fatal(`"b main." + Tab returned no candidates`)
	}
	for _, want := range []string{"b main.main", "b main.helper"} {
		if !hasMatch(res.Matches, want) {
			t.Fatalf("missing %q in %v", want, res.Matches)
		}
	}
	// Ambiguous prefix: the line must stay at the common prefix so the wildmenu opens.
	if res.Completion != "b main." {
		t.Fatalf("completion: got %q want %q", res.Completion, "b main.")
	}
}
