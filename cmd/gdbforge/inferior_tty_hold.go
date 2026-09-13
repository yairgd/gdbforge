package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// inferiorTTYHoldFlag selects the hold helper: gdbforge re-executes itself
// inside the external terminal to keep that window open for program stdio.
const inferiorTTYHoldFlag = "--hold-inferior-tty"

// wantsInferiorTTYHold reports whether argv selects the hold helper.
func wantsInferiorTTYHold(args []string) bool {
	return len(args) > 0 && args[0] == inferiorTTYHoldFlag
}

// inferiorTTYHoldShell builds the shell command the external terminal runs to
// hold its window. `exec` keeps the terminal's direct child pid, which the
// helper needs to release the pts (see runInferiorTTYHold).
func inferiorTTYHoldShell(pathFile, pidFile string) string {
	exe, err := os.Executable()
	if err == nil {
		if _, serr := os.Stat(exe); serr != nil {
			err = serr
		}
	}
	if err != nil {
		// Last resort: a plain shell keeps the window open, but the pts stays
		// this session's controlling terminal, so the program gets none.
		return fmt.Sprintf("echo $$ > %s; tty > %s; exec sleep infinity",
			shellSingleQuote(pidFile), shellSingleQuote(pathFile))
	}
	return fmt.Sprintf("exec %s %s %s %s", shellSingleQuote(exe), inferiorTTYHoldFlag,
		shellSingleQuote(pathFile), shellSingleQuote(pidFile))
}

// runInferiorTTYHold holds the external terminal window open and hands its pts
// to the debugged program, then waits until gdbforge kills it.
//
// It must run as the terminal emulator's direct child (via `exec`), because it
// releases the pts with TIOCNOTTY and the kernel only detaches the terminal
// from the session when the caller is the session leader. Without that release
// the pts stays the controlling terminal of this session, the inferior's
// TIOCSCTTY fails with EPERM ("GDB: Failed to set controlling terminal"), and
// the program runs with no controlling terminal at all — open("/dev/tty")
// returns ENXIO, which breaks Go TUIs (tcell, bubbletea), curses and getpass.
//
// argv: <pathFile> <pidFile>. Both are written only after the release, so
// gdbforge never points -inferior-tty-set at a pts that is still claimed.
func runInferiorTTYHold(args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: gdbforge %s <path-file> <pid-file>\n", inferiorTTYHoldFlag)
		return 2
	}
	pathFile, pidFile := args[1], args[2]

	fd, pts, err := holdTTY()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gdbforge: cannot resolve this terminal: %v\n", err)
		return 1
	}

	if note := releaseControllingTTY(fd); note != "" {
		fmt.Fprintln(os.Stdout, "gdbforge: "+note)
	}
	fmt.Fprintf(os.Stdout, "gdbforge: program stdio → %s (close with :set inferior-tty internal)\n\n", pts)

	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "gdbforge: write %s: %v\n", pidFile, err)
		return 1
	}
	if err := os.WriteFile(pathFile, []byte(pts), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "gdbforge: write %s: %v\n", pathFile, err)
		return 1
	}

	// gdbforge signals the helper when :set inferior-tty internal closes the
	// window; nothing else should end the wait.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	return 0
}

// releaseControllingTTY detaches this session from the terminal on fd so the
// debugged program can claim it. It returns a human-readable note when the
// release did not happen, and "" on success.
func releaseControllingTTY(fd int) string {
	noCtty := " — the program will run without a controlling terminal"
	if sid, err := unix.Getsid(0); err == nil && sid != os.Getpid() {
		return fmt.Sprintf("not this terminal's session leader (sid %d, pid %d)%s", sid, os.Getpid(), noCtty)
	}
	// TIOCNOTTY hangs up the foreground process group, which is this process.
	signal.Ignore(syscall.SIGHUP, syscall.SIGTTOU)
	defer signal.Reset(syscall.SIGHUP, syscall.SIGTTOU)

	if err := unix.IoctlSetInt(fd, unix.TIOCNOTTY, 0); err != nil {
		return fmt.Sprintf("could not release this terminal (%v)%s", err, noCtty)
	}
	return ""
}

// holdTTY finds the terminal the emulator handed us and its /dev/pts/N path.
func holdTTY() (int, string, error) {
	for _, fd := range []int{0, 1, 2} {
		if _, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ); err != nil {
			continue
		}
		if p, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd)); err == nil && strings.HasPrefix(p, "/dev/") {
			return fd, p, nil
		}
		p, err := ttyPathViaCommand(fd) // no /proc (BSD): ask coreutils
		if err != nil {
			return 0, "", err
		}
		return fd, p, nil
	}
	return 0, "", fmt.Errorf("stdin, stdout and stderr are not a terminal")
}

func ttyPathViaCommand(fd int) (string, error) {
	// Duplicate: os.NewFile attaches a finalizer that would close our stdio.
	dup, err := unix.Dup(fd)
	if err != nil {
		return "", fmt.Errorf("dup tty fd: %w", err)
	}
	f := os.NewFile(uintptr(dup), "tty")
	defer f.Close()

	cmd := exec.Command("tty")
	cmd.Stdin = f
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tty: %w", err)
	}
	p := strings.TrimSpace(string(out))
	if !strings.HasPrefix(p, "/dev/") {
		return "", fmt.Errorf("tty reported %q", p)
	}
	return p, nil
}
