//go:build linux

package termui

import (
	"os/signal"
	"runtime"
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

// stopForShellJobControl stops this process for shell job control and does not
// return until the shell resumes it (SIGCONT / fg). Call only after
// screen.Suspend().
//
// The stop must be synchronous. kill(getpid(), SIGTSTP) is process-directed:
// the kernel may hand the signal to any other thread and let this one keep
// running, so the caller races ahead into the resume path and re-enters raw
// mode before the group stop lands. The shell then resets the terminal while we
// are stopped, and after fg nothing re-applies raw mode. A thread-directed
// signal to the current thread is delivered before tgkill returns, so lock the
// goroutine to its thread, unblock SIGTSTP there, and signal that thread.
func stopForShellJobControl() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// A blocked SIGTSTP would only go pending and let us run on.
	_ = unix.PthreadSigmask(unix.SIG_UNBLOCK, &tstpSigMask, nil)
	signal.Reset(syscall.SIGTSTP)
	return unix.Tgkill(unix.Getpid(), unix.Gettid(), unix.SIGTSTP)
}
