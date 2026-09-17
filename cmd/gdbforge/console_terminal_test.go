package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubPath puts fake executables named bins in a temp dir and makes it the
// whole PATH, so lookups resolve the same way on any machine.
func stubPath(t *testing.T, bins ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, b := range bins {
		if err := os.WriteFile(filepath.Join(dir, b), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	return dir
}

func TestConsoleTerminalShellMinicom(t *testing.T) {
	stubPath(t, "minicom", "screen")
	shell, err := consoleTerminalShell("/dev/pts/7", 115200)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shell, "minicom") {
		t.Fatalf("expected minicom, got %q", shell)
	}
	if !strings.Contains(shell, "minirc.dfl") {
		t.Fatalf("expected minirc.dfl config path, got %q", shell)
	}
	if strings.Contains(shell, "-C ") {
		t.Fatalf("must not use minicom -C (capture file), got %q", shell)
	}
	if !strings.Contains(shell, "/dev/pts/7") {
		t.Fatalf("missing pty in %q", shell)
	}
}

func TestConsoleTerminalShellScreenFallback(t *testing.T) {
	stubPath(t, "screen")
	shell, err := consoleTerminalShell("/dev/pts/7", 115200)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shell, "screen") || !strings.Contains(shell, "/dev/pts/7") {
		t.Fatalf("expected screen on the pty, got %q", shell)
	}
}

func TestConsoleTerminalShellNoTool(t *testing.T) {
	stubPath(t)
	if _, err := consoleTerminalShell("/dev/pts/7", 115200); err == nil {
		t.Fatal("expected an error when neither minicom nor screen is installed")
	}
}

func TestTerminalRunArgvMateTerminal(t *testing.T) {
	stubPath(t, "minicom", "mate-terminal")
	t.Setenv("GDBFORGE_TERMINAL", "mate-terminal")
	shell, err := consoleTerminalShell("/dev/pts/7", 115200)
	if err != nil {
		t.Fatal(err)
	}
	argv, err := terminalRunArgv([]string{"sh", "-c", shell})
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) < 3 || argv[0] != "mate-terminal" || argv[1] != "-e" {
		t.Fatalf("argv: %v", argv)
	}
	cmd := argv[2]
	if !strings.HasPrefix(cmd, "sh -c ") {
		t.Fatalf("mate-terminal must use sh -c, got %q", cmd)
	}
	if !strings.Contains(cmd, "minicom") || !strings.Contains(cmd, "/dev/pts/7") {
		t.Fatalf("missing minicom args in %q", cmd)
	}
}
