package gdb

import (
	"fmt"
	"os/exec"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

// GDBClient is a GDB MI session over a PTY, plus a separate inferior TTY for
// the debugged program's stdin/stdout (-inferior-tty-set).
// QuitGate holds MI quit confirmation policy (UI paints from QuitAction).
type GDBClient struct {
	*ptyx.Client
	inferior *ptyx.TTY
	Quit     QuitGate
}

var _ core.Session = (*GDBClient)(nil)

func NewGDBClient(gdbPath, prog string, progArgs ...string) (*GDBClient, error) {
	if gdbPath == "" {
		gdbPath = "gdb"
	}
	if prog == "" {
		return nil, fmt.Errorf("program path is required")
	}

	var argv []string
	if len(progArgs) > 0 {
		argv = append([]string{gdbPath, "--interpreter=mi2", "--args", prog}, progArgs...)
	} else {
		argv = []string{gdbPath, "--interpreter=mi2", prog}
	}

	if _, err := exec.LookPath(gdbPath); err != nil {
		return nil, fmt.Errorf("find %s: %w", gdbPath, err)
	}

	gdbPty, err := ptyx.New(argv, ptyx.Options{})
	if err != nil {
		return nil, err
	}

	inf, err := ptyx.OpenTTY()
	if err != nil {
		gdbPty.Close()
		return nil, err
	}

	c := &GDBClient{Client: gdbPty, inferior: inf}
	_ = c.Send("")
	// Route the program's stdio to the inferior PTY before any run/exec.
	_ = c.Send(fmt.Sprintf("-inferior-tty-set %s", inf.SlaveName()))
	return c, nil
}

// InferiorTTY returns the program I/O PTY (master held by gdbforge), or nil.
func (c *GDBClient) InferiorTTY() *ptyx.TTY {
	if c == nil {
		return nil
	}
	return c.inferior
}

// RequestQuit applies Ctrl-D / EOF quit policy (confirm or send q).
func (c *GDBClient) RequestQuit() QuitAction {
	if c == nil {
		return QuitSendQ
	}
	return c.Quit.RequestQuit()
}

// Interrupt sends Ctrl-C on the GDB MI PTY (same as console C-c).
func (c *GDBClient) Interrupt() error {
	if c == nil {
		return nil
	}
	return c.SendRaw("\x03")
}

// Close tears down the inferior TTY then the GDB PTY session.
func (c *GDBClient) Close() {
	if c == nil {
		return
	}
	if c.inferior != nil {
		c.inferior.Close()
		c.inferior = nil
	}
	if c.Client != nil {
		c.Client.Close()
	}
}
