package gdb

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

// GDBClient owns three PTYs: CLI (user console), MI (backend), and inferior
// (program stdio). Session I/O uses the MI PTY after bootstrap via new-ui.
type GDBClient struct {
	*ptyx.TTY // MI PTY — core.Session
	CLI       *ptyx.TTY
	inferior  *ptyx.TTY
	Quit      QuitGate
	startupOut string
}

// ClientOptions configures NewGDBClient.
type ClientOptions struct {
	// InferiorTTY, if non-empty, is used for -inferior-tty-set instead of
	// opening an internal ptyx.TTY (external terminal / TUI debug).
	InferiorTTY string
}

var _ core.Session = (*GDBClient)(nil)

// How long to wait for the first (gdb) prompt after spawn (covers -x remote load).
const startupPromptWait = 90 * time.Second

// miAsyncOn puts the MI UI in background execution mode.
//
// With mi-async off (GDB's default) a running target blocks the MI UI entirely:
// GDB stops reading the MI channel, so -exec-interrupt, break and clear sit
// unread in the pty until the target happens to stop on its own. Before new-ui
// this was masked, because MI was GDB's controlling terminal and a ^C byte on it
// reached GDB as SIGINT through the line discipline. The MI pty has no
// foreground process group, so that escape hatch is gone and async is required.
// Targets that cannot do async are unaffected — GDB keeps running them synchronously.
const miAsyncOn = "-gdb-set mi-async on"

func NewGDBClient(gdbPath string, gdbArgs []string) (*GDBClient, error) {
	return NewGDBClientOpts(gdbPath, gdbArgs, ClientOptions{})
}

// NewGDBClientOpts starts GDB in console mode on PTY #1, adds MI on PTY #2 via
// new-ui mi2, and routes inferior I/O to PTY #3 (or AttachPath).
func NewGDBClientOpts(gdbPath string, gdbArgs []string, opts ClientOptions) (*GDBClient, error) {
	if gdbPath == "" {
		gdbPath = "gdb"
	}

	argv := []string{gdbPath}
	if !gdbArgsHasPaginationOff(gdbArgs) {
		argv = append(argv, "-iex", "set pagination off")
	}
	argv = append(argv, gdbArgs...)

	if _, err := exec.LookPath(gdbPath); err != nil {
		return nil, fmt.Errorf("cannot find debugger %q: %w", gdbPath, err)
	}

	cli, err := ptyx.Start(argv, ptyx.Options{})
	if err != nil {
		return nil, err
	}

	mi, err := ptyx.Open()
	if err != nil {
		cli.Close()
		return nil, err
	}

	c := &GDBClient{CLI: cli, TTY: mi}

	ext := strings.TrimSpace(opts.InferiorTTY)
	if ext != "" {
		c.inferior = ptyx.AttachPath(ext)
	} else {
		inf, err := ptyx.Open()
		if err != nil {
			c.Close()
			return nil, err
		}
		c.inferior = inf
	}

	out, err := waitTTYPrompt(cli, startupPromptWait)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.startupOut = out

	if err := cli.Send(fmt.Sprintf("new-ui mi2 %s", mi.SlaveName())); err != nil {
		c.Close()
		return nil, fmt.Errorf("new-ui mi2: %w", err)
	}

	if _, err := waitTTYPrompt(mi, startupPromptWait); err != nil {
		c.Close()
		return nil, fmt.Errorf("mi ui ready: %w", err)
	}

	if err := mi.Send(miAsyncOn); err != nil {
		c.Close()
		return nil, fmt.Errorf("%s: %w", miAsyncOn, err)
	}

	if path := c.InferiorTTYPath(); path != "" {
		if err := c.ConfigureInferiorTTY(); err != nil {
			c.Close()
			return nil, err
		}
	}

	return c, nil
}

// CLITTY returns the GDB console PTY wired to the GDB terminal widget.
func (c *GDBClient) CLITTY() *ptyx.TTY {
	if c == nil {
		return nil
	}
	return c.CLI
}

// MITTY returns the MI backend PTY.
func (c *GDBClient) MITTY() *ptyx.TTY {
	if c == nil {
		return nil
	}
	return c.TTY
}

// TakeStartupOutput returns and clears bytes captured through the first CLI prompt.
func (c *GDBClient) TakeStartupOutput() string {
	if c == nil {
		return ""
	}
	s := c.startupOut
	c.startupOut = ""
	return s
}

// ConfigureInferiorTTY routes program stdio to the inferior path via MI.
func (c *GDBClient) ConfigureInferiorTTY() error {
	if c == nil {
		return nil
	}
	path := c.InferiorTTYPath()
	if path == "" {
		return nil
	}
	return c.Send(fmt.Sprintf("-inferior-tty-set %s", path))
}

// InferiorTTYPath returns the path passed to -inferior-tty-set.
func (c *GDBClient) InferiorTTYPath() string {
	if c == nil || c.inferior == nil {
		return ""
	}
	return c.inferior.SlaveName()
}

// UsesExternalInferiorTTY reports whether stdio is an external terminal path.
func (c *GDBClient) UsesExternalInferiorTTY() bool {
	return c != nil && c.inferior != nil && c.inferior.IsExternal()
}

// SetInferiorTTYPath switches -inferior-tty-set to path.
func (c *GDBClient) SetInferiorTTYPath(path string) error {
	if c == nil {
		return fmt.Errorf("no gdb session")
	}
	path = strings.TrimSpace(path)
	if path == "" || path == "internal" {
		if c.inferior == nil || c.inferior.IsExternal() {
			if c.inferior != nil && c.inferior.HasMaster() {
				c.inferior.Close()
			}
			inf, err := ptyx.Open()
			if err != nil {
				return err
			}
			c.inferior = inf
		}
		return c.ConfigureInferiorTTY()
	}
	if c.inferior != nil && c.inferior.HasMaster() {
		c.inferior.Close()
	}
	c.inferior = ptyx.AttachPath(path)
	return c.ConfigureInferiorTTY()
}

func waitTTYPrompt(tty *ptyx.TTY, timeout time.Duration) (string, error) {
	if tty == nil {
		return "", fmt.Errorf("no tty")
	}
	ch, cancel := tty.Subscribe()
	defer cancel()
	deadline := time.After(timeout)
	var buf strings.Builder
	for {
		select {
		case <-deadline:
			return buf.String(), fmt.Errorf("timeout waiting for gdb prompt after startup (check -x script / remote)")
		case msg, ok := <-ch:
			if !ok {
				return buf.String(), fmt.Errorf("gdb exited before prompt")
			}
			if msg.Err != nil {
				return buf.String(), msg.Err
			}
			if msg.Data == "" {
				continue
			}
			buf.WriteString(msg.Data)
			if strings.Contains(buf.String(), MIPromptToken) {
				return buf.String(), nil
			}
			if buf.Len() > 256*1024 {
				s := buf.String()
				buf.Reset()
				if len(s) > 64*1024 {
					buf.WriteString(s[len(s)-64*1024:])
				} else {
					buf.WriteString(s)
				}
			}
		}
	}
}

// HasInitScript reports whether gdbArgs include -x / -ex style command files.
func HasInitScript(gdbArgs []string) bool {
	for _, a := range gdbArgs {
		switch {
		case a == "-x" || a == "-ex" || a == "-iex" || a == "-ix":
			return true
		case strings.HasPrefix(a, "-x=") || strings.HasPrefix(a, "-ex=") ||
			strings.HasPrefix(a, "-iex=") || strings.HasPrefix(a, "-ix="):
			return true
		case a == "--command" || a == "--eval-command":
			return true
		}
	}
	return false
}

func gdbArgsHasPaginationOff(gdbArgs []string) bool {
	for i, a := range gdbArgs {
		low := strings.ToLower(strings.TrimSpace(a))
		if strings.Contains(low, "set pagination off") || strings.Contains(low, "set height 0") ||
			strings.Contains(low, "set height unlimited") {
			return true
		}
		if (a == "-iex" || a == "-ex" || a == "-ix" || a == "-x") && i+1 < len(gdbArgs) {
			next := strings.ToLower(strings.TrimSpace(gdbArgs[i+1]))
			if strings.Contains(next, "set pagination off") || strings.Contains(next, "set height 0") ||
				strings.Contains(next, "set height unlimited") {
				return true
			}
		}
	}
	return false
}

// InferiorTTY returns the in-process inferior PTY master, or nil when external.
func (c *GDBClient) InferiorTTY() *ptyx.TTY {
	if c == nil || c.inferior == nil || !c.inferior.HasMaster() {
		return nil
	}
	return c.inferior
}

// RequestQuit applies Ctrl-D / EOF quit policy.
func (c *GDBClient) RequestQuit() QuitAction {
	if c == nil {
		return QuitSendQ
	}
	return c.Quit.RequestQuit()
}

// Interrupt stops a running inferior.
//
// new-ui makes the MI UI the owner of execution while the CLI UI sits at its own
// (gdb) prompt. GDB attributes a SIGINT / ^C on the CLI terminal to that idle UI,
// so it only prints "Quit" and the target keeps running. -exec-interrupt on the
// MI channel is what actually stops it; the terminal signal stays as a fallback
// for when there is no MI channel.
func (c *GDBClient) Interrupt() error {
	if c == nil {
		return nil
	}
	if c.TTY != nil {
		if err := c.TTY.Send(MIExecInterrupt); err == nil {
			return nil
		}
	}
	return c.InterruptIdle()
}

// InterruptIdle delivers ^C to GDB's own terminal, which is what a Ctrl-C at an
// idle prompt should do ("Quit"). -exec-interrupt would answer with an error
// because no program is being run.
func (c *GDBClient) InterruptIdle() error {
	if c == nil || c.CLI == nil {
		return nil
	}
	err := c.CLI.SignalInterrupt()
	if sendErr := c.CLI.SendRaw("\x03"); err == nil {
		err = sendErr
	}
	return err
}

// SuspendInferior delivers SIGTSTP to the inferior process group.
func (c *GDBClient) SuspendInferior() error {
	if c == nil {
		return nil
	}
	if err := signalInferiorTSTP(c.Quit.InferiorPID()); err == nil {
		return nil
	}
	if inf := c.InferiorTTY(); inf != nil {
		return inf.SendRaw("\x1a")
	}
	return fmt.Errorf("suspend inferior: no pid and no tty")
}

func signalInferiorTSTP(pidStr string) error {
	if pidStr == "" {
		return fmt.Errorf("no inferior pid")
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return fmt.Errorf("bad inferior pid %q", pidStr)
	}
	if err := syscall.Kill(-pid, syscall.SIGTSTP); err == nil {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGTSTP)
}

// Close tears down CLI, MI, and inferior PTYs.
func (c *GDBClient) Close() {
	if c == nil {
		return
	}
	if c.inferior != nil && c.inferior.HasMaster() {
		c.inferior.Close()
		c.inferior = nil
	}
	if c.CLI != nil {
		c.CLI.Close()
		c.CLI = nil
	}
	if c.TTY != nil {
		c.TTY.Close()
		c.TTY = nil
	}
}
