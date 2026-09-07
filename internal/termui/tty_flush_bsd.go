//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package termui

import (
	"os"

	"golang.org/x/sys/unix"
)

// freadFlag is FREAD from <sys/fcntl.h>, which x/sys/unix does not export.
// TIOCFLUSH takes a pointer to these flags rather than the termios queue
// selector Linux uses for TCFLSH.
const freadFlag = 0x1

// flushControllingTTYInput discards bytes typed at the shell prompt while
// gdbforge was SIGSTOP'd. CompositeTerminal forwards keyboard to PTY masters;
// without this flush, stale input corrupts GDB/IO after fg.
func flushControllingTTYInput() {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return
	}
	defer f.Close()
	_ = unix.IoctlSetPointerInt(int(f.Fd()), unix.TIOCFLUSH, freadFlag)
}
