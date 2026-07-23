//go:build linux || aix || solaris || zos

package ptyx

import "golang.org/x/sys/unix"

// Linux and System V-style termios ioctls.
const (
	ioctlReadTermios  = unix.TCGETS
	ioctlWriteTermios = unix.TCSETS
)
