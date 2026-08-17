package main

import (
	"strings"
	"testing"
)

func TestConsoleTerminalShellMinicom(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
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

func TestTerminalRunArgvMateTerminal(t *testing.T) {
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
