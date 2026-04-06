package gdb

import (
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/yairgd/promptcore/internal/core"
	"golang.org/x/sys/unix"
)

type GDBClient struct {
	cmd  *exec.Cmd
	ptmx *os.File
}

func NewGDBClient() (*GDBClient, error) {

	// cmd := exec.Command("gdb", "--interpreter=mi")
	cmd := exec.Command("gdb", "hello")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	_ = setRaw(int(ptmx.Fd()))

	return &GDBClient{
		cmd:  cmd,
		ptmx: ptmx,
	}, nil
}

// Start reads from PTY and pushes structured messages
func (c *GDBClient) Start(output chan<- core.GdbOutputMsg) {
	go func() {
		buf := make([]byte, 1024)

		for {
			n, err := c.ptmx.Read(buf)

			if n > 0 {
				//	if len(buf) > 0 && buf[0] == '\n' {
				//		buf = buf[1:]
				//	}

				output <- core.GdbOutputMsg{
					Data: string(buf[:n]),
				}
			}

			if err != nil {
				if err != io.EOF {
					output <- core.GdbOutputMsg{Err: err}
				}
				close(output)
				return
			}
		}
	}()
}

func (c *GDBClient) Send(cmd string) error {
	_, err := c.ptmx.Write([]byte(cmd + "\n"))
	return err
}

func (c *GDBClient) SendRaw(s string) error {
	_, err := c.ptmx.Write([]byte(s))
	return err
}

func (c *GDBClient) Close() {
	if c.ptmx != nil {
		_ = c.ptmx.Close()
	}

	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
}

func setRaw(fd int) error {
	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return err
	}

	// disable echo
	termios.Lflag &^= unix.ECHO

	// disable canonical mode (optional but recommended)
	termios.Lflag &^= unix.ICANON

	// apply
	return unix.IoctlSetTermios(fd, unix.TCSETS, termios)
}
