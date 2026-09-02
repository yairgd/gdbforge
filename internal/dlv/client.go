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
	PromptToken    = "(dlv)"
	PromptLiveHost = PromptToken + " "

	startupPromptWait = 30 * time.Second
)

// Client is an interactive Delve session over a PTY, plus a separate inferior
// TTY for the debugged program's stdin/stdout when supported.
type Client struct {
	*ptyx.TTY
	inferior *ptyx.TTY
	startupOut string
}

// ClientOptions configures NewClient.
type ClientOptions struct {
	InferiorTTY string
}

var _ core.Session = (*Client)(nil)

func NewClient(dlvPath string, dlvArgs []string) (*Client, error) {
	return NewClientOpts(dlvPath, dlvArgs, ClientOptions{})
}

func NewClientOpts(dlvPath string, dlvArgs []string, opts ClientOptions) (*Client, error) {
	if dlvPath == "" {
		dlvPath = "dlv"
	}
	if len(dlvArgs) == 0 {
		return nil, fmt.Errorf("dlv arguments required (program path)")
	}

	if _, err := exec.LookPath(dlvPath); err != nil {
		return nil, fmt.Errorf("cannot find debugger %q: %w", dlvPath, err)
	}

	c := &Client{}
	ext := strings.TrimSpace(opts.InferiorTTY)
	ttyPath := ext
	if ext != "" {
		c.inferior = ptyx.AttachPath(ext)
	} else {
		inf, err := ptyx.Open()
		if err != nil {
			return nil, err
		}
		c.inferior = inf
		ttyPath = inf.SlaveName()
	}

	argv := []string{dlvPath, "exec", "--tty", ttyPath, "--"}
	argv = append(argv, dlvArgs...)

	env := filterEnv(os.Environ(), "PAGER", "DELVE_PAGER")
	env = append(env, "PAGER=cat", "DELVE_PAGER=cat")

	dlvPty, err := ptyx.Start(argv, ptyx.Options{Env: env})
	if err != nil {
		if c.inferior != nil && c.inferior.HasMaster() {
			c.inferior.Close()
		}
		return nil, err
	}

	c.TTY = dlvPty
	out, err := c.waitForPrompt(startupPromptWait)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.startupOut = out
	return c, nil
}

func (c *Client) TakeStartupOutput() string {
	if c == nil {
		return ""
	}
	s := c.startupOut
	c.startupOut = ""
	return s
}

func NewConnectClient(dlvPath, addr string) (*Client, error) {
	if dlvPath == "" {
		dlvPath = "dlv"
	}
	addr = normalizeConnectAddr(addr)
	if addr == "" {
		return nil, fmt.Errorf("dlv connect: address required")
	}
	if _, err := exec.LookPath(dlvPath); err != nil {
		return nil, fmt.Errorf("cannot find debugger %q: %w", dlvPath, err)
	}

	env := filterEnv(os.Environ(), "PAGER", "DELVE_PAGER")
	env = append(env, "PAGER=cat", "DELVE_PAGER=cat")

	dlvPty, err := ptyx.Start([]string{dlvPath, "connect", addr}, ptyx.Options{Env: env})
	if err != nil {
		return nil, err
	}

	c := &Client{
		TTY:      dlvPty,
		inferior: ptyx.AttachPath("headless:" + addr),
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
	if !strings.Contains(addr, ":") {
		return "127.0.0.1:" + addr
	}
	return addr
}

func (c *Client) ConfigureInferiorTTY() error { return nil }

func (c *Client) InferiorTTYPath() string {
	if c == nil || c.inferior == nil {
		return ""
	}
	return c.inferior.SlaveName()
}

func (c *Client) UsesExternalInferiorTTY() bool {
	return c != nil && c.inferior != nil && c.inferior.IsExternal()
}

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

func (c *Client) InferiorTTY() *ptyx.TTY {
	if c == nil || c.inferior == nil || !c.inferior.HasMaster() {
		return nil
	}
	return c.inferior
}

func (c *Client) waitForPrompt(timeout time.Duration) (string, error) {
	if c == nil || c.TTY == nil {
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

func (c *Client) Interrupt() error {
	if c == nil {
		return nil
	}
	err := c.SignalInterrupt()
	_ = c.SendRaw("\x03")
	return err
}

func (c *Client) Close() {
	if c == nil {
		return
	}
	if c.inferior != nil && c.inferior.HasMaster() {
		c.inferior.Close()
		c.inferior = nil
	}
	if c.TTY != nil {
		c.TTY.Close()
		c.TTY = nil
	}
}
