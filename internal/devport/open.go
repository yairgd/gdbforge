// Package devport opens UART and PTY device paths via go.bug.st/serial (8N1).
package devport

import (
	"io"

	"go.bug.st/serial"
)

// Port is a readable, writable, closable device (hardware UART or /dev/pts/N).
type Port = io.ReadWriteCloser

// Open opens device at baud (raw 8N1). Works for /dev/ttyUSB* and /dev/pts/*.
func Open(device string, baud int) (Port, error) {
	return serial.Open(device, &serial.Mode{
		BaudRate: baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
}
