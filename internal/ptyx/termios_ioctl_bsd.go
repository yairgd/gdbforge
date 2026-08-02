//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package ptyx

import "golang.org/x/sys/unix"

// BSD / Darwin termios ioctls (TCGETS/TCSETS are Linux-only).
const (
	ioctlReadTermios  = unix.TIOCGETA
	ioctlWriteTermios = unix.TIOCSETA
)
