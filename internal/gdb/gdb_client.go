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

// GDBClient is a GDB MI session over a PTY, plus a separate inferior TTY for
// the debugged program's stdin/stdout (-inferior-tty-set).
// QuitGate holds MI quit confirmation policy (UI paints from QuitAction).
type GDBClient struct {
	*ptyx.Client
	inferior    *ptyx.TTY
	externalTTY string // when set, -inferior-tty-set uses this path (no internal master)
	Quit        QuitGate
	startupOut  string // PTY bytes through first (gdb); for UI after NewGDBClient
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

func NewGDBClient(gdbPath string, gdbArgs []string) (*GDBClient, error) {
	return NewGDBClientOpts(gdbPath, gdbArgs, ClientOptions{})
}

// NewGDBClientOpts is NewGDBClient with optional external inferior TTY path.
func NewGDBClientOpts(gdbPath string, gdbArgs []string, opts ClientOptions) (*GDBClient, error) {
	if gdbPath == "" {
		gdbPath = "gdb"
	}

	// Always MI2; disable pagination before -x / load so long "Loading section…"
	// output is not stuck on "--Type <RET> for more…" (no UI yet to answer).
	argv := []string{gdbPath, "--interpreter=mi2"}
	if !gdbArgsHasPaginationOff(gdbArgs) {
		argv = append(argv, "-iex", "set pagination off")
	}
	argv = append(argv, gdbArgs...)

	if _, err := exec.LookPath(gdbPath); err != nil {
		return nil, fmt.Errorf("cannot find debugger %q: %w", gdbPath, err)
	}

	gdbPty, err := ptyx.New(argv, ptyx.Options{})
	if err != nil {
		return nil, err
	}

	c := &GDBClient{Client: gdbPty}
	ext := strings.TrimSpace(opts.InferiorTTY)
	if ext != "" {
		c.externalTTY = ext
	} else {
		inf, err := ptyx.OpenTTY()
		if err != nil {
			gdbPty.Close()
			return nil, err
		}
		c.inferior = inf
	}

	out, err := c.waitForPrompt(startupPromptWait)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.startupOut = out
	return c, nil
}

// TakeStartupOutput returns and clears bytes captured through the first prompt.
func (c *GDBClient) TakeStartupOutput() string {
	if c == nil {
		return ""
	}
	s := c.startupOut
	c.startupOut = ""
	return s
}

// ConfigureInferiorTTY routes the program's stdio to the inferior TTY path
// (internal slave or external /dev/pts/N). Call after the console bridge is
// subscribed so the MI reply is visible.
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
	if c == nil {
		return ""
	}
	if c.externalTTY != "" {
		return c.externalTTY
	}
	if c.inferior != nil {
		return c.inferior.SlaveName()
	}
	return ""
}

// UsesExternalInferiorTTY reports whether stdio is an external terminal path
// (no in-process master for the IO widget).
func (c *GDBClient) UsesExternalInferiorTTY() bool {
	return c != nil && c.externalTTY != ""
}

// SetInferiorTTYPath switches -inferior-tty-set to path. Empty or "internal"
// reopens an in-process PTY. Non-empty path closes the internal master (if any)
// and uses the external slave (TUI / other terminal).
func (c *GDBClient) SetInferiorTTYPath(path string) error {
	if c == nil {
		return fmt.Errorf("no gdb session")
	}
	path = strings.TrimSpace(path)
	if path == "" || path == "internal" {
		c.externalTTY = ""
		if c.inferior == nil {
			inf, err := ptyx.OpenTTY()
			if err != nil {
				return err
			}
			c.inferior = inf
		}
		return c.ConfigureInferiorTTY()
	}
	if c.inferior != nil {
		c.inferior.Close()
		c.inferior = nil
	}
	c.externalTTY = path
	return c.ConfigureInferiorTTY()
}

func (c *GDBClient) waitForPrompt(timeout time.Duration) (string, error) {
	if c == nil || c.Client == nil {
		return "", fmt.Errorf("no gdb session")
	}
	ch, cancel := c.Subscribe()
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
			// Cap buffer so huge -x chatter does not grow forever.
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

// HasInitScript reports whether gdbArgs include -x / -ex style command files
// (used to skip gdbforge's default "break main").
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

// gdbArgsHasPaginationOff reports whether the user already disables pagination.
func gdbArgsHasPaginationOff(gdbArgs []string) bool {
	for i, a := range gdbArgs {
		low := strings.ToLower(strings.TrimSpace(a))
		if strings.Contains(low, "set pagination off") || strings.Contains(low, "set height 0") ||
			strings.Contains(low, "set height unlimited") {
			return true
		}
		// -iex/-ex followed by the command as next argv
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

// Interrupt stops a running inferior via PTY ^C and SIGINT to the GDB process.
//
// Do not lead with -exec-interrupt: while GDB is blocked in continue, typed MI
// is often unread until the prompt returns (Send still succeeds) — that was a
// Ctrl-C regression. ^C is the classic, reliable GDB interrupt.
func (c *GDBClient) Interrupt() error {
	if c == nil {
		return nil
	}
	err := c.SignalInterrupt()
	if sendErr := c.SendRaw("\x03"); err == nil {
		err = sendErr
	}
	return err
}

// SuspendInferior delivers SIGTSTP like a terminal Ctrl-Z on the program's TTY.
// Prefers kill(pid, SIGTSTP): writing ^Z alone often only echoes "^Z" on :b io
// when the inferior is not the PTY foreground group. Falls back to PTY ^Z.
func (c *GDBClient) SuspendInferior() error {
	if c == nil {
		return nil
	}
	if err := signalInferiorTSTP(c.Quit.InferiorPID()); err == nil {
		return nil
	}
	if c.inferior != nil {
		return c.inferior.SendRaw("\x1a")
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
	// Negative pid = process group (usual shell job-control target).
	if err := syscall.Kill(-pid, syscall.SIGTSTP); err == nil {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGTSTP)
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
