//go:build !linux

package serial

import "fmt"

// Open is only supported on Linux.
func Open(device string, baud int) (*Port, error) {
	_ = device
	_ = baud
	return nil, fmt.Errorf("serial: unsupported on this platform")
}
