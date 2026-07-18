package gdb

import (
	"fmt"
	"os/exec"

	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/ptyx"
)

// GDBClient is a GDB MI session over a shared PTY client.
type GDBClient struct {
	*ptyx.Client
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

	pty, err := ptyx.New(argv, ptyx.Options{})
	if err != nil {
		return nil, err
	}
	c := &GDBClient{Client: pty}
	_ = c.Send("")
	return c, nil
}
