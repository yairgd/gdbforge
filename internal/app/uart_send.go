package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/yairgd/termforge/devport"
)

func writePayload(w io.WriteCloser, device string, payload []byte) error {
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("uart_send %s: %w", device, err)
	}
	return nil
}

func sendUartLine(device string, baud int, line string) error {
	device = strings.TrimSpace(device)
	line = strings.TrimSpace(line)
	if device == "" {
		return fmt.Errorf("uart_send: empty device")
	}
	if line == "" {
		return fmt.Errorf("uart_send: empty line")
	}
	payload := []byte(line + "\r\n")

	port, err := devport.Open(device, baud)
	if err != nil {
		return err
	}
	defer port.Close()
	return writePayload(port, device, payload)
}
