//go:build linux

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

// holdHelperEnv re-enters the hold helper in a child copy of the test binary,
// which is the only way to exercise it: the helper detaches its session from
// its controlling terminal, so it must not run in the test process.
const holdHelperEnv = "GDBFORGE_TEST_HOLD_HELPER"

// ttyProbeEnv re-enters the test binary as a stand-in for GDB's inferior: it
// only reports whether it got a controlling terminal.
const ttyProbeEnv = "GDBFORGE_TEST_TTY_PROBE"

func TestMain(m *testing.M) {
	if v := os.Getenv(holdHelperEnv); v != "" {
		args := strings.Split(v, string(os.PathListSeparator))
		os.Exit(runInferiorTTYHold(append([]string{inferiorTTYHoldFlag}, args...)))
	}
	if os.Getenv(ttyProbeEnv) != "" {
		f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			os.Exit(3)
		}
		f.Close()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestInferiorTTYHoldReleasesControllingTerminal is the regression test for
// "GDB: Failed to set controlling terminal: Operation not permitted" plus
// open("/dev/tty") = ENXIO in the inferior: while the hold process keeps the
// pts as its controlling terminal, GDB's inferior cannot take it over.
func TestInferiorTTYHoldReleasesControllingTerminal(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("no pty: %v", err)
	}
	defer master.Close()
	defer slave.Close()
	go io.Copy(io.Discard, master) // keep the pts buffer from filling up

	dir := t.TempDir()
	pathFile := filepath.Join(dir, "tty")
	pidFile := filepath.Join(dir, "pid")

	// Setsid+Setctty is what a terminal emulator does to its child.
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), holdHelperEnv+"="+pathFile+string(os.PathListSeparator)+pidFile)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start hold helper: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	deadline := time.Now().Add(10 * time.Second)
	var gotPath, gotPID string
	for time.Now().Before(deadline) {
		p, perr := os.ReadFile(pathFile)
		q, qerr := os.ReadFile(pidFile)
		if perr == nil && qerr == nil && len(p) > 0 && len(q) > 0 {
			gotPath, gotPID = strings.TrimSpace(string(p)), strings.TrimSpace(string(q))
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if gotPath == "" {
		t.Fatal("hold helper never advertised its pts")
	}
	if gotPath != slave.Name() {
		t.Fatalf("advertised pts %q, want %q", gotPath, slave.Name())
	}
	if pid, err := strconv.Atoi(gotPID); err != nil || pid != cmd.Process.Pid {
		t.Fatalf("pid file %q, want %d", gotPID, cmd.Process.Pid)
	}

	// The advertised pts must be claimable, exactly as GDB's new_tty() does it:
	// setsid + TIOCSCTTY, then open("/dev/tty").
	if err := claimTTYAndOpenDevTTY(gotPath); err != nil {
		t.Fatalf("inferior could not take over %s: %v", gotPath, err)
	}

	// The window must stay open: gdbforge kills the helper on
	// :set inferior-tty internal, nothing else.
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("hold helper died: %v", err)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal hold helper: %v", err)
	}
	done := make(chan error, 1)
	go func() { _, err := cmd.Process.Wait(); done <- err }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("hold helper ignored SIGTERM")
	}
}

// claimTTYAndOpenDevTTY mimics GDB's inferior: a fresh session takes pts over as
// its controlling terminal (TIOCSCTTY, which Go does for Setctty) and opens
// /dev/tty. Both steps fail while another session still owns the pts.
func claimTTYAndOpenDevTTY(pts string) error {
	f, err := os.OpenFile(pts, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), ttyProbeEnv+"=1")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = f, f, f
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("TIOCSCTTY: %w", err) // EPERM when the pts is still claimed
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("open /dev/tty in the new session: %w", err)
	}
	return nil
}

func TestInferiorTTYHoldShellExecsSelf(t *testing.T) {
	got := inferiorTTYHoldShell("/tmp/p a t h", "/tmp/pid")
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("no executable path: %v", err)
	}
	// `exec` keeps the terminal's child pid, which the helper needs to be the
	// session leader when it releases the pts.
	if !strings.HasPrefix(got, "exec "+shellSingleQuote(exe)+" "+inferiorTTYHoldFlag+" ") {
		t.Fatalf("hold shell = %q", got)
	}
	if !strings.Contains(got, shellSingleQuote("/tmp/p a t h")) {
		t.Fatalf("path file not quoted in %q", got)
	}
}

func TestWantsInferiorTTYHold(t *testing.T) {
	if !wantsInferiorTTYHold([]string{inferiorTTYHoldFlag, "/tmp/a", "/tmp/b"}) {
		t.Fatal("hold flag not recognized")
	}
	for _, args := range [][]string{nil, {"./hello"}, {"-g", "dlv"}, {"--", inferiorTTYHoldFlag}} {
		if wantsInferiorTTYHold(args) {
			t.Fatalf("args %v should not select the hold helper", args)
		}
	}
}
