//go:build linux

package termui

import (
	"os"

	"golang.org/x/sys/unix"
)

// flushControllingTTYInput discards bytes typed at the shell prompt while
// gdbforge was SIGSTOP'd. CompositeTerminal forwards keyboard to PTY masters;
// without this flush, stale input corrupts GDB/IO after fg.
func flushControllingTTYInput() {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return
	}
	defer f.Close()
	_ = unix.IoctlSetInt(int(f.Fd()), unix.TCFLSH, unix.TCIFLUSH)
}
