package dlv

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

const (
	// PromptToken is the interactive Delve CLI prompt.
	PromptToken = "(dlv)"
	// PromptLiveHost is PromptToken plus one trailing space for the caret.
	PromptLiveHost = PromptToken + " "

	startupPromptWait = 30 * time.Second
)

// Client is an interactive Delve session over a PTY, plus a separate inferior
// TTY for the debugged program's stdin/stdout when supported.
type Client struct {
	*ptyx.Client
	inferior   *ptyx.TTY
	startupOut string
}

var _ core.Session = (*Client)(nil)

// NewClient spawns `dlv exec --tty <slave> -- prog [args...]` so the inferior's
// stdin/stdout use a dedicated PTY (same dual-PTY model as GDB's
// -inferior-tty-set), painted in the IO / Output widget.
func NewClient(dlvPath string, dlvArgs []string) (*Client, error) {
	if dlvPath == "" {
		dlvPath = "dlv"
	}
	if len(dlvArgs) == 0 {
		return nil, fmt.Errorf("dlv arguments required (program path)")
	}

	if _, err := exec.LookPath(dlvPath); err != nil {
		return nil, fmt.Errorf("find %s: %w", dlvPath, err)
	}

	inf, err := ptyx.OpenTTY()
	if err != nil {
		return nil, err
	}

	// --tty must be a Delve flag (before "--"); program args follow "--".
	argv := []string{dlvPath, "exec", "--tty", inf.SlaveName(), "--"}
	argv = append(argv, dlvArgs...)

	// Force a dumb pager so `stack` / `goroutines` are not trapped in less
	// (alt-screen + END prompts break Query capture and flood the UI queue).
	// Replace existing PAGER* entries: execve consumers often take the first
	// occurrence, so appending alone does not override the parent environment.
	env := filterEnv(os.Environ(), "PAGER", "DELVE_PAGER")
	env = append(env, "PAGER=cat", "DELVE_PAGER=cat")

	dlvPty, err := ptyx.New(argv, ptyx.Options{Env: env})
	if err != nil {
		inf.Close()
		return nil, err
	}

	c := &Client{Client: dlvPty, inferior: inf}
	out, err := c.waitForPrompt(startupPromptWait)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.startupOut = out
	return c, nil
}

// TakeStartupOutput returns and clears bytes captured through the first prompt.
func (c *Client) TakeStartupOutput() string {
	if c == nil {
		return ""
	}
	s := c.startupOut
	c.startupOut = ""
	return s
}

// ConfigureInferiorTTY is a no-op: the inferior TTY is already attached via
// `dlv exec --tty <slave>` at spawn time (peer of GDB -inferior-tty-set).
func (c *Client) ConfigureInferiorTTY() error {
	return nil
}

func (c *Client) waitForPrompt(timeout time.Duration) (string, error) {
	if c == nil || c.Client == nil {
		return "", fmt.Errorf("no dlv session")
	}
	ch, cancel := c.Subscribe()
	defer cancel()
	deadline := time.After(timeout)
	var buf strings.Builder
	for {
		select {
		case <-deadline:
			return buf.String(), fmt.Errorf("timeout waiting for dlv prompt after startup")
		case msg, ok := <-ch:
			if !ok {
				return buf.String(), fmt.Errorf("dlv exited before prompt")
			}
			if msg.Err != nil {
				return buf.String(), msg.Err
			}
			if msg.Data == "" {
				continue
			}
			buf.WriteString(msg.Data)
			if strings.Contains(buf.String(), PromptToken) {
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

// filterEnv drops entries whose key (before '=') is in drop (case-sensitive).
func filterEnv(env []string, drop ...string) []string {
	if len(env) == 0 || len(drop) == 0 {
		return env
	}
	deny := make(map[string]struct{}, len(drop))
	for _, k := range drop {
		deny[k] = struct{}{}
	}
	out := make([]string, 0, len(env))
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		if _, skip := deny[key]; skip {
			continue
		}
		out = append(out, e)
	}
	return out
}

// InferiorTTY returns the program I/O PTY, or nil.
func (c *Client) InferiorTTY() *ptyx.TTY {
	if c == nil {
		return nil
	}
	return c.inferior
}

// Interrupt sends Ctrl-C on the Delve PTY.
func (c *Client) Interrupt() error {
	if c == nil {
		return nil
	}
	return c.SendRaw("\x03")
}

// Close tears down the inferior TTY then the Delve PTY session.
func (c *Client) Close() {
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
