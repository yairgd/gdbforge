// Package ptyx provides a shared PTY session used by GDB and exec backends.
package ptyx

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/yairgd/gdbforge/internal/core"
	"golang.org/x/sys/unix"
)

const (
	readBufSize    = 32 * 1024
	outputChanSize = 64
)

// Client owns a process attached to a PTY.
// Writes are exclusive (one holder via WithWrite / Send); Subscribe fans out
// output to every reader (UI, MCP, …).
type Client struct {
	cmd  *exec.Cmd
	ptmx *os.File

	writeMu sync.Mutex

	subMu  sync.Mutex
	subs   map[chan core.PtyOutputMsg]struct{}
	closed bool

	closeOnce sync.Once
}

var _ core.Session = (*Client)(nil)

// Options configure New.
type Options struct {
	// Rows/Cols set the initial winsize (0 = leave default / unset).
	Rows, Cols uint16
	// Env replaces the child environment when non-nil (like exec.Cmd.Env).
	// When nil, the child inherits the parent environment.
	Env []string
}

// New starts argv on a PTY, sets raw mode, and begins the reader goroutine.
func New(argv []string, opt Options) (*Client, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("command is required")
	}
	name := argv[0]
	if _, err := exec.LookPath(name); err != nil {
		return nil, fmt.Errorf("find %s: %w", name, err)
	}

	cmd := exec.Command(name, argv[1:]...)
	if opt.Env != nil {
		cmd.Env = opt.Env
	}
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", name, err)
	}
	_ = setRaw(int(ptmx.Fd()))
	if opt.Rows > 0 || opt.Cols > 0 {
		rows, cols := opt.Rows, opt.Cols
		if rows == 0 {
			rows = 24
		}
		if cols == 0 {
			cols = 80
		}
		_ = pty.Setsize(ptmx, &pty.Winsize{Rows: rows, Cols: cols})
	}

	c := &Client{
		cmd:  cmd,
		ptmx: ptmx,
		subs: make(map[chan core.PtyOutputMsg]struct{}),
	}
	c.startReader()
	return c, nil
}

// Subscribe registers for PTY output chunks. cancel removes the subscription
// and closes the channel. All subscriptions are closed on Close().
// Drain promptly — a full buffer drops that chunk for that subscriber only.
func (c *Client) Subscribe() (<-chan core.PtyOutputMsg, func()) {
	ch := make(chan core.PtyOutputMsg, outputChanSize)

	c.subMu.Lock()
	if c.closed {
		c.subMu.Unlock()
		close(ch)
		return ch, func() {}
	}
	c.subs[ch] = struct{}{}
	c.subMu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			c.subMu.Lock()
			if _, ok := c.subs[ch]; ok {
				delete(c.subs, ch)
				close(ch)
			}
			c.subMu.Unlock()
		})
	}
	return ch, cancel
}

func (c *Client) broadcast(msg core.PtyOutputMsg) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	for ch := range c.subs {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (c *Client) startReader() {
	ptmx := c.ptmx
	go func() {
		defer c.dropSubs()
		buf := make([]byte, readBufSize)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				c.broadcast(core.PtyOutputMsg{Data: string(buf[:n])})
			}
			if err != nil {
				if err != io.EOF {
					c.broadcast(core.PtyOutputMsg{Err: err})
				}
				return
			}
		}
	}()
}

// dropSubs closes every subscription channel (PTY EOF / reader exit).
func (c *Client) dropSubs() {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	if c.closed {
		return
	}
	for ch := range c.subs {
		close(ch)
	}
	c.subs = make(map[chan core.PtyOutputMsg]struct{})
}

type writeGate struct{ c *Client }

func (g writeGate) Send(cmd string) error {
	return g.c.writeRawUnlocked(cmd + "\n")
}

func (g writeGate) SendRaw(s string) error {
	return g.c.writeRawUnlocked(s)
}

func (c *Client) writeRawUnlocked(s string) error {
	if c.ptmx == nil {
		return io.ErrClosedPipe
	}
	_, err := c.ptmx.Write([]byte(s))
	return err
}

// WithWrite holds the exclusive PTY write lock for the duration of fn.
// Only one writer (UI submit or MCP GdbCommand) runs at a time; readers
// via Subscribe continue to receive output.
func (c *Client) WithWrite(ctx context.Context, fn func(w core.PTYWriter) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(writeGate{c})
}

func (c *Client) Send(cmd string) error {
	return c.WithWrite(context.Background(), func(w core.PTYWriter) error {
		return w.Send(cmd)
	})
}

func (c *Client) SendRaw(s string) error {
	return c.WithWrite(context.Background(), func(w core.PTYWriter) error {
		return w.SendRaw(s)
	})
}

// SetSize updates the PTY window size (SSH / curses / pane resize).
func (c *Client) SetSize(rows, cols uint16) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.ptmx == nil {
		return io.ErrClosedPipe
	}
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}
	return pty.Setsize(c.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.writeMu.Lock()
		if c.ptmx != nil {
			_ = c.ptmx.Close()
			c.ptmx = nil
		}
		c.writeMu.Unlock()

		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
			_, _ = c.cmd.Process.Wait()
			c.cmd = nil
		}

		c.subMu.Lock()
		c.closed = true
		for ch := range c.subs {
			close(ch)
		}
		c.subs = nil
		c.subMu.Unlock()
	})
}

func setRaw(fd int) error {
	termios, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		return err
	}

	termios.Lflag &^= unix.ECHO
	termios.Lflag &^= unix.ICANON

	return unix.IoctlSetTermios(fd, ioctlWriteTermios, termios)
}
