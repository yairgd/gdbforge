package gdb

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/yairgd/cgdb-go/internal/core"
	"golang.org/x/sys/unix"
)

const (
	ptyReadBufSize = 32 * 1024
	outputChanSize = 64
)

type GDBClient struct {
	cmd  *exec.Cmd
	ptmx *os.File

	closeOnce sync.Once
}

func NewGDBClient(gdbPath, prog string, progArgs ...string) (*GDBClient, chan core.GdbOutputMsg, error) {
	if gdbPath == "" {
		gdbPath = "gdb"
	}
	if prog == "" {
		return nil, nil, fmt.Errorf("program path is required")
	}

	var args []string
	if len(progArgs) > 0 {
		args = append([]string{"--interpreter=mi2", "--args", prog}, progArgs...)
	} else {
		args = []string{"--interpreter=mi2", prog}
	}

	if _, err := exec.LookPath(gdbPath); err != nil {
		return nil, nil, fmt.Errorf("find %s: %w", gdbPath, err)
	}

	cmd := exec.Command(gdbPath, args...)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, nil, fmt.Errorf("start %s: %w", gdbPath, err)
	}
	_ = setRaw(int(ptmx.Fd()))

	client := &GDBClient{
		cmd:  cmd,
		ptmx: ptmx,
	}

	outputChan := make(chan core.GdbOutputMsg, outputChanSize)
	client.Start(outputChan)
	_ = client.Send("")

	return client, outputChan, nil
}

// Start reads from the PTY and pushes output chunks with minimal copies.
func (c *GDBClient) Start(output chan<- core.GdbOutputMsg) {
	go func() {
		defer close(output)

		buf := make([]byte, ptyReadBufSize)
		for {
			n, err := c.ptmx.Read(buf)
			if n > 0 {
				// string([]byte) copies once; avoid an extra intermediate []byte.
				output <- core.GdbOutputMsg{Data: string(buf[:n])}
			}
			if err != nil {
				if err != io.EOF {
					output <- core.GdbOutputMsg{Err: err}
				}
				return
			}
		}
	}()
}

func (c *GDBClient) Send(cmd string) error {
	if c.ptmx == nil {
		return io.ErrClosedPipe
	}
	_, err := c.ptmx.Write([]byte(cmd + "\n"))
	return err
}

func (c *GDBClient) SendRaw(s string) error {
	if c.ptmx == nil {
		return io.ErrClosedPipe
	}
	_, err := c.ptmx.Write([]byte(s))
	return err
}

func (c *GDBClient) Close() {
	c.closeOnce.Do(func() {
		if c.ptmx != nil {
			_ = c.ptmx.Close()
			c.ptmx = nil
		}
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
			_, _ = c.cmd.Process.Wait()
			c.cmd = nil
		}
	})
}

func setRaw(fd int) error {
	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return err
	}

	termios.Lflag &^= unix.ECHO
	termios.Lflag &^= unix.ICANON

	return unix.IoctlSetTermios(fd, unix.TCSETS, termios)
}
