package dlv

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/go-delve/delve/service/rpc2"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

const (
	PromptToken    = "(dlv)"
	PromptLiveHost = PromptToken + " "

	startupPromptWait = 30 * time.Second

	// Initial `dlv connect` winsize. Delve's liner needs a non-zero column
	// count at startup to enable its line editor.
	connectPTYRows = 24
	connectPTYCols = 80
)

// Client is an interactive Delve session: CLI PTY (`dlv connect`) for human
// I/O plus an optional rpc2 client for machine control when attached to a
// headless server. Inferior program I/O uses a separate TTY (--tty).
type Client struct {
	*ptyx.TTY
	inferior   *ptyx.TTY
	RPC        *rpc2.RPCClient
	headless   *exec.Cmd
	listenAddr string
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

	ext := strings.TrimSpace(opts.InferiorTTY)
	return newHeadlessRPCClient(dlvPath, dlvArgs, ext)
}

// newHeadlessRPCClient runs headless dlv exec + rpc2 + dlv connect. When extTTY
// is set, program stdio uses that path (owned pts from OpenExternalTTY); otherwise
// gdbforge allocates an internal pair for the IO pane.
func newHeadlessRPCClient(dlvPath string, dlvArgs []string, extTTY string) (*Client, error) {
	c := &Client{}
	var ttyPath string
	if extTTY != "" {
		c.inferior = ptyx.AttachPath(extTTY)
		ttyPath = extTTY
	} else {
		inf, err := ptyx.Open()
		if err != nil {
			return nil, err
		}
		c.inferior = inf
		ttyPath = inf.SlaveName()
	}

	listenAddr, err := PickListenAddr()
	if err != nil {
		c.closeInferior()
		return nil, err
	}

	env := delveEnv()
	headArgs := headlessExecArgv(dlvPath, listenAddr, ttyPath, dlvArgs)
	headCmd := exec.Command(headArgs[0], headArgs[1:]...)
	headCmd.Env = env
	headCmd.Stdout = io.Discard
	headCmd.Stderr = io.Discard
	headCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := headCmd.Start(); err != nil {
		c.closeInferior()
		return nil, fmt.Errorf("dlv headless: %w", err)
	}
	c.headless = headCmd
	c.listenAddr = listenAddr

	rpc, err := DialRPC(listenAddr, startupPromptWait)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.RPC = rpc

	cli, err := startConnectPTY(dlvPath, listenAddr, env)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.TTY = cli

	out, err := c.waitForPrompt(startupPromptWait)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.startupOut = out
	return c, nil
}

func headlessExecArgv(dlvPath, listenAddr, ttyPath string, dlvArgs []string) []string {
	argv := []string{
		dlvPath, "exec",
		"--headless",
		"--listen=" + listenAddr,
		"--api-version=2",
		"--accept-multiclient",
		"--tty", ttyPath,
		"--",
	}
	return append(argv, dlvArgs...)
}

func startConnectPTY(dlvPath, addr string, env []string) (*ptyx.TTY, error) {
	addr = normalizeConnectAddr(addr)
	// Rows/Cols must be set before exec: Delve's liner reads the winsize once at
	// startup, and a 0-column PTY makes every Prompt fall back to its "too narrow"
	// dumb reader (no arrow keys, history, or Tab). The pane resizes it on paint.
	dlvPty, err := ptyx.Start([]string{dlvPath, "connect", addr}, ptyx.Options{
		Env:  env,
		Rows: connectPTYRows,
		Cols: connectPTYCols,
	})
	if err != nil {
		return nil, err
	}
	return dlvPty, nil
}

func delveEnv() []string {
	env := filterEnv(os.Environ(), "PAGER", "DELVE_PAGER")
	return append(env, "PAGER=cat", "DELVE_PAGER=cat")
}

func (c *Client) TakeStartupOutput() string {
	if c == nil {
		return ""
	}
	s := c.startupOut
	c.startupOut = ""
	return s
}

// ListenAddr returns the headless server address when known.
func (c *Client) ListenAddr() string {
	if c == nil {
		return ""
	}
	return c.listenAddr
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

	rpc, err := DialRPC(addr, startupPromptWait)
	if err != nil {
		return nil, err
	}

	env := delveEnv()
	cli, err := startConnectPTY(dlvPath, addr, env)
	if err != nil {
		_ = rpc.Detach(false)
		return nil, err
	}

	c := &Client{
		TTY:        cli,
		RPC:        rpc,
		listenAddr: addr,
		inferior:   ptyx.AttachPath("headless:" + addr),
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

// ListFunctionsFilter returns function names matching a regex filter via rpc2.
// Completion must never write `funcs …` to the CLI PTY: that PTY carries the
// user's half-typed line, so the query text would be appended to it.
func (c *Client) ListFunctionsFilter(filter string) ([]string, error) {
	if c == nil || c.RPC == nil {
		return nil, fmt.Errorf("dlv: no rpc2 client")
	}
	return c.RPC.ListFunctions(filter, 0)
}

func (c *Client) Interrupt() error {
	if c == nil {
		return nil
	}
	// Headless multiclient: halt via rpc2 only. Sending ^C to the connect CLI
	// triggers Delve's [p/q]? gate instead of returning to (dlv).
	if c.RPC != nil {
		_, err := c.RPC.Halt()
		return err
	}
	err := c.SignalInterrupt()
	_ = c.SendRaw("\x03")
	return err
}

func (c *Client) closeInferior() {
	if c != nil && c.inferior != nil && c.inferior.HasMaster() {
		c.inferior.Close()
		c.inferior = nil
	}
}

func (c *Client) Close() {
	if c == nil {
		return
	}
	if c.RPC != nil {
		_ = c.RPC.Detach(true)
		c.RPC = nil
	}
	if c.headless != nil && c.headless.Process != nil {
		_ = c.headless.Process.Kill()
		c.headless = nil
	}
	c.closeInferior()
	if c.TTY != nil {
		c.TTY.Close()
		c.TTY = nil
	}
}
