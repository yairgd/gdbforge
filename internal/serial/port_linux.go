//go:build linux

package serial

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Open opens device exclusively in raw 8N1 mode at the given baud rate.
func Open(device string, baud int) (*Port, error) {
	if device == "" {
		return nil, fmt.Errorf("serial: empty device")
	}
	fd, err := unix.Open(device, unix.O_RDWR|unix.O_NOCTTY|unix.O_EXCL, 0)
	if err != nil {
		return nil, fmt.Errorf("serial open %s: %w", device, err)
	}
	f := os.NewFile(uintptr(fd), device)
	if err := configureRaw(f, baud); err != nil {
		_ = f.Close()
		return nil, err
	}
	_ = setModemLines(f)
	return &Port{rw: f, name: device}, nil
}

func setModemLines(f *os.File) error {
	bits, err := unix.IoctlGetInt(int(f.Fd()), unix.TIOCMGET)
	if err != nil {
		return nil // some USB adapters omit modem ioctls
	}
	bits |= unix.TIOCM_DTR | unix.TIOCM_RTS
	return unix.IoctlSetInt(int(f.Fd()), unix.TIOCMSET, bits)
}

func configureRaw(f *os.File, baud int) error {
	t, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	if err != nil {
		return fmt.Errorf("serial termios get: %w", err)
	}
	rate, ok := baudConstant(baud)
	if !ok {
		return fmt.Errorf("serial: unsupported baud %d", baud)
	}
	t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON | unix.IXOFF |
		unix.IXANY | unix.IMAXBEL
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ISIG | unix.ICANON | unix.ECHO | unix.ECHOE | unix.ECHOK |
		unix.ECHONL | unix.IEXTEN
	t.Cflag &^= unix.CSIZE | unix.PARENB | unix.CSTOPB | unix.CRTSCTS
	t.Cflag |= unix.CS8 | unix.CREAD | unix.CLOCAL
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
	t.Ispeed = rate
	t.Ospeed = rate
	if err := unix.IoctlSetTermios(int(f.Fd()), unix.TCSETS, t); err != nil {
		return fmt.Errorf("serial termios set: %w", err)
	}
	return nil
}

func baudConstant(baud int) (uint32, bool) {
	switch baud {
	case 9600:
		return unix.B9600, true
	case 19200:
		return unix.B19200, true
	case 38400:
		return unix.B38400, true
	case 57600:
		return unix.B57600, true
	case 115200:
		return unix.B115200, true
	case 230400:
		return unix.B230400, true
	case 460800:
		return unix.B460800, true
	case 921600:
		return unix.B921600, true
	default:
		return 0, false
	}
}
