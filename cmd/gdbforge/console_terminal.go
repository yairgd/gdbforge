package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// consoleTerminalShell returns a shell snippet that runs a serial UI on pty.
// Minicom gets a config with HW/SW flow control off (required for PTY slaves).
func consoleTerminalShell(pty string, baud int) (string, error) {
	if pty == "" {
		return "", fmt.Errorf("console: empty pty")
	}
	if baud <= 0 {
		baud = 115200
	}
	if _, err := exec.LookPath("minicom"); err == nil {
		return minicomShell(pty, baud)
	}
	if _, err := exec.LookPath("screen"); err == nil {
		return fmt.Sprintf("exec screen %s %d", shellSingleQuote(pty), baud), nil
	}
	return "", fmt.Errorf("console: install minicom or screen")
}

func minicomShell(pty string, baud int) (string, error) {
	dir, err := gdbforgeMinicomConfigDir()
	if err != nil {
		return "", err
	}
	cfgPath := filepath.Join(dir, "minirc.dfl")
	cfg := fmt.Sprintf(
		"pu port             %s\npu baudrate         %d\npu rtscts           No\npu xonxoff          No\npu modem            No\n",
		pty, baud,
	)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		return "", err
	}
	// Config file is a positional arg (-C is capture-file on current minicom, not config dir).
	return fmt.Sprintf(
		"exec minicom -D %s -b %d -o -w %s",
		shellSingleQuote(pty), baud, shellSingleQuote(cfgPath),
	), nil
}

func gdbforgeMinicomConfigDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "gdbforge")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func spawnConsoleTerminal(app *DebuggerApp, pty string, baud int) error {
	shell, err := consoleTerminalShell(pty, baud)
	if err != nil {
		return err
	}
	return app.SpawnTerminal([]string{"sh", "-c", shell})
}
