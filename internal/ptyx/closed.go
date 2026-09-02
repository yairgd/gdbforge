package ptyx

import (
	"errors"
	"io"
	"strings"
	"syscall"
)

// ClosedError reports PTY reader errors that mean the session ended (debugger
// quit, process death, slave closed). Linux often returns EIO on /dev/ptmx.
func ClosedError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == syscall.EIO {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "input/output error") ||
		strings.Contains(msg, "file already closed")
}
