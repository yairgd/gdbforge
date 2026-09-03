//go:build unix

package termui

import (
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

var tstpSigMask unix.Sigset_t

func init() {
	sigsetAdd(&tstpSigMask, int(unix.SIGTSTP))
}

func sigsetAdd(set *unix.Sigset_t, sig int) {
	if set == nil || sig < 1 {
		return
	}
	set.Val[sig/64] |= 1 << (uint(sig) % 64)
}

// blockJobControlStop masks SIGTSTP while tcell owns the alt-screen. Ctrl-Z is
// handled as a tcell key; the kernel must not stop us mid-disengage.
func blockJobControlStop() {
	_ = unix.PthreadSigmask(unix.SIG_BLOCK, &tstpSigMask, nil)
}

func unblockJobControlStop() {
	_ = unix.PthreadSigmask(unix.SIG_UNBLOCK, &tstpSigMask, nil)
}

// stopForShellJobControl disengages the TUI first, then stops for shell job
// control (fg to resume). Call only after screen.Suspend().
func stopForShellJobControl() error {
	unblockJobControlStop()
	signal.Reset(syscall.SIGTSTP)
	return unix.Kill(unix.Getpid(), unix.SIGTSTP)
}
