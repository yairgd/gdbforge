//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package termui

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// orphanGuard bounds the wait for SIGCONT. A stop signal sent to an orphaned
// process group is discarded, so without a bound Ctrl-Z would hang the UI
// thread whenever gdbforge runs without job control.
const orphanGuard = 2 * time.Second

// stopForShellJobControl stops this process for shell job control and does not
// return until the shell resumes it (SIGCONT / fg). Call only after
// screen.Suspend().
//
// The stop must be synchronous. kill(getpid(), SIGTSTP) is process-directed:
// the kernel may hand the signal to any other thread and let this one keep
// running, so the caller races ahead into the resume path and re-enters raw
// mode before the group stop lands. The shell then resets the terminal while we
// are stopped, and after fg nothing re-applies raw mode. There is no
// thread-directed signal here without cgo (pthread_kill), so wait for the
// SIGCONT instead: the only statement between the stop request and the terminal
// work is a receive, which parks this thread even if it wins the race.
func stopForShellJobControl() error {
	cont := make(chan os.Signal, 1)
	signal.Notify(cont, syscall.SIGCONT)
	defer signal.Stop(cont)

	signal.Reset(syscall.SIGTSTP)
	if err := unix.Kill(unix.Getpid(), unix.SIGTSTP); err != nil {
		return err
	}

	timer := time.NewTimer(orphanGuard)
	defer timer.Stop()
	select {
	case <-cont:
	case <-timer.C:
		// Either the stop was discarded, or the guard expired while we were
		// stopped and both are ready now. Drain so a real SIGCONT is not left
		// pending for the next Ctrl-Z.
		select {
		case <-cont:
		default:
		}
	}
	return nil
}
