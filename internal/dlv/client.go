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
	inferior    *ptyx.TTY
	externalTTY string // --tty path when not using an internal master
	startupOut  string
}

// ClientOptions configures NewClient.
type ClientOptions struct {
	// InferiorTTY, if non-empty, is passed as `dlv exec --tty` instead of
	// opening an internal ptyx.TTY.
	InferiorTTY string
}

var _ core.Session = (*Client)(nil)

// NewClient spawns `dlv exec --tty <slave> -- prog [args...]` so the inferior's
// stdin/stdout use a dedicated PTY (same dual-PTY model as GDB's
// -inferior-tty-set), painted in the IO / Output widget unless InferiorTTY is
// an external path.
func NewClient(dlvPath string, dlvArgs []string) (*Client, error) {
	return NewClientOpts(dlvPath, dlvArgs, ClientOptions{})
}

// NewClientOpts is NewClient with optional external inferior TTY path.
func NewClientOpts(dlvPath string, dlvArgs []string, opts ClientOptions) (*Client, error) {
	if dlvPath == "" {
		dlvPath = "dlv"
	}
	if len(dlvArgs) == 0 {
		return nil, fmt.Errorf("dlv arguments required (program path)")
	}

	if _, err := exec.LookPath(dlvPath); err != nil {
		return nil, fmt.Errorf("find %s: %w", dlvPath, err)
	}

	c := &Client{}
	ext := strings.TrimSpace(opts.InferiorTTY)
	ttyPath := ext
	if ext != "" {
		c.externalTTY = ext
	} else {
		inf, err := ptyx.OpenTTY()
		if err != nil {
			return nil, err
		}
		c.inferior = inf
		ttyPath = inf.SlaveName()
	}

	// --tty must be a Delve flag (before "--"); program args follow "--".
	argv := []string{dlvPath, "exec", "--tty", ttyPath, "--"}
	argv = append(argv, dlvArgs...)

	env := filterEnv(os.Environ(), "PAGER", "DELVE_PAGER")
	env = append(env, "PAGER=cat", "DELVE_PAGER=cat")

	dlvPty, err := ptyx.New(argv, ptyx.Options{Env: env})
	if err != nil {
		if c.inferior != nil {
			c.inferior.Close()
		}
		return nil, err
	}

	c.Client = dlvPty
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

// NewConnectClient spawns `dlv connect <addr>` on a PTY (terminal client to a
// headless Delve server). Inferior stdio stays with the headless process /
// its terminal — no local --tty.
func NewConnectClient(dlvPath, addr string) (*Client, error) {
	if dlvPath == "" {
		dlvPath = "dlv"
	}
	addr = normalizeConnectAddr(addr)
	if addr == "" {
		return nil, fmt.Errorf("dlv connect: address required")
	}
	if _, err := exec.LookPath(dlvPath); err != nil {
		return nil, fmt.Errorf("find %s: %w", dlvPath, err)
	}

	env := filterEnv(os.Environ(), "PAGER", "DELVE_PAGER")
	env = append(env, "PAGER=cat", "DELVE_PAGER=cat")

	dlvPty, err := ptyx.New([]string{dlvPath, "connect", addr}, ptyx.Options{Env: env})
	if err != nil {
		return nil, err
	}

	c := &Client{
		Client:      dlvPty,
		externalTTY: "headless:" + addr,
	}
	out, err := c.waitForPrompt(startupPromptWait)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.startupOut = out
	return c, nil
}

func normalizeConnectAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.HasPrefix(addr, "unix:") {
		return addr
	}
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	// bare port
	if !strings.Contains(addr, ":") {
		return "127.0.0.1:" + addr
	}
	return addr
}

// ConfigureInferiorTTY is a no-op after spawn: the inferior TTY is already
// attached via `dlv exec --tty <slave>` (peer of GDB -inferior-tty-set).
func (c *Client) ConfigureInferiorTTY() error {
	return nil
}

// InferiorTTYPath returns the --tty path used for the inferior.
func (c *Client) InferiorTTYPath() string {
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

// UsesExternalInferiorTTY reports whether stdio is an external terminal path.
func (c *Client) UsesExternalInferiorTTY() bool {
	return c != nil && c.externalTTY != ""
}

// SetInferiorTTYPath is unused by the app: Delve --tty is applied only at
// spawn, so DebuggerApp restarts the session instead. Kept for API symmetry.
func (c *Client) SetInferiorTTYPath(path string) error {
	path = strings.TrimSpace(path)
	cur := c.InferiorTTYPath()
	if path == "" || path == "internal" {
		if c.UsesExternalInferiorTTY() {
			return fmt.Errorf("dlv inferior tty requires session restart")
		}
		return nil
	}
	if path == cur {
		return nil
	}
	return fmt.Errorf("dlv inferior tty requires session restart")
}

// InferiorTTY returns the program I/O PTY, or nil when using an external path.
func (c *Client) InferiorTTY() *ptyx.TTY {
	if c == nil {
		return nil
	}
	return c.inferior
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

// Interrupt stops a running target.
//
// While Delve's CLI is blocked in continue/next/step, typed commands such as
// "halt" are not read until the prompt returns — only ^C / SIGINT unblocks it.
// SIGINT is sent first (no dependency on the PTY write lock), then a PTY ^C.
func (c *Client) Interrupt() error {
	if c == nil {
		return nil
	}
	err := c.SignalInterrupt()
	_ = c.SendRaw("\x03")
	return err
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
