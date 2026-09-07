// Package ptyx provides PTY sessions for debugger CLIs, inferior I/O, and exec.
package ptyx

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/yairgd/gdbforge/internal/core"
)

const (
	readBufSize    = 32 * 1024
	outputChanSize = 64
)

type subscriber struct {
	ch        chan core.PtyOutputMsg
	done      chan struct{}
	closeOnce sync.Once
}

func (sub *subscriber) closeDone() {
	sub.closeOnce.Do(func() { close(sub.done) })
}

// Options configure Start.
type Options struct {
	// Rows/Cols set the initial winsize (0 = leave default / unset).
	Rows, Cols uint16
	// Env replaces the child environment when non-nil (like exec.Cmd.Env).
	// When nil, the child inherits the parent environment.
	Env []string
}

// TTY holds a PTY master (when present), optional child process, and fan-out
// readers. Create with Start (process), Open (new pair), or AttachPath (external
// slave path only — metadata for -inferior-tty-set / --tty).
type TTY struct {
	master    *os.File
	slave     *os.File // kept open in Open mode so the pts node stays valid
	slaveName string
	cmd       *exec.Cmd // non-nil for Start mode

	writeMu sync.Mutex

	subMu  sync.Mutex
	subs   map[*subscriber]struct{}
	closed bool

	closeOnce sync.Once
}

var _ core.Session = (*TTY)(nil)

// Open allocates a master/slave PTY pair for inferior or MI I/O.
func Open() (*TTY, error) {
	return openPair()
}

// OpenTTY is deprecated; use Open.
func OpenTTY() (*TTY, error) {
	return Open()
}

func openPair() (*TTY, error) {
	master, slave, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("open pty: %w", err)
	}
	name := slave.Name()
	if name == "" {
		_ = master.Close()
		_ = slave.Close()
		return nil, fmt.Errorf("pty: empty slave name")
	}
	t := &TTY{
		master:    master,
		slave:     slave,
		slaveName: name,
		subs:      make(map[*subscriber]struct{}),
	}
	t.startReader()
	return t, nil
}

// AttachPath records an external device path (e.g. /dev/pts/N) without opening
// it in-process. HasMaster is false; Subscribe delivers nothing.
func AttachPath(path string) *TTY {
	path = strings.TrimSpace(path)
	return &TTY{
		slaveName: path,
		subs:      make(map[*subscriber]struct{}),
	}
}

// Start runs argv on a PTY and begins the reader goroutine.
func Start(argv []string, opt Options) (*TTY, error) {
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

	t := &TTY{
		master:    ptmx,
		slaveName: "", // process PTY — no separate slave fd kept
		cmd:       cmd,
		subs:      make(map[*subscriber]struct{}),
	}
	t.startReader()
	return t, nil
}

// HasMaster reports whether this TTY holds an in-process master fd.
func (t *TTY) HasMaster() bool {
	return t != nil && t.master != nil
}

// IsExternal reports a path-only TTY (external terminal / TUI debug target).
func (t *TTY) IsExternal() bool {
	return t != nil && t.master == nil && t.slaveName != ""
}

// SlaveName is the path for -inferior-tty-set or dlv exec --tty.
func (t *TTY) SlaveName() string {
	if t == nil {
		return ""
	}
	return t.slaveName
}

// Master returns the in-process PTY master fd (nil for AttachPath-only TTYs).
func (t *TTY) Master() *os.File {
	if t == nil {
		return nil
	}
	return t.master
}

func (t *TTY) Subscribe() (<-chan core.PtyOutputMsg, func()) {
	ch := make(chan core.PtyOutputMsg, outputChanSize)

	t.subMu.Lock()
	if t.closed || !t.HasMaster() {
		t.subMu.Unlock()
		close(ch)
		return ch, func() {}
	}

	sub := &subscriber{
		ch:   ch,
		done: make(chan struct{}),
	}
	t.subs[sub] = struct{}{}
	t.subMu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			t.subMu.Lock()
			delete(t.subs, sub)
			t.subMu.Unlock()
			sub.closeDone()
		})
	}
	return ch, cancel
}

func (t *TTY) broadcast(msg core.PtyOutputMsg) {
	t.subMu.Lock()
	subs := make([]*subscriber, 0, len(t.subs))
	for sub := range t.subs {
		subs = append(subs, sub)
	}
	t.subMu.Unlock()

	for _, sub := range subs {
		select {
		case sub.ch <- msg:
		case <-sub.done:
		}
	}
}

func (t *TTY) startReader() {
	if !t.HasMaster() {
		return
	}
	master := t.master
	go func() {
		defer t.dropSubs()
		buf := make([]byte, readBufSize)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				t.broadcast(core.PtyOutputMsg{Data: string(buf[:n])})
			}
			if err != nil {
				if err != io.EOF {
					t.broadcast(core.PtyOutputMsg{Err: err})
				}
				return
			}
		}
	}()
}

func (t *TTY) dropSubs() {
	t.subMu.Lock()
	defer t.subMu.Unlock()

	for sub := range t.subs {
		sub.closeDone()
		close(sub.ch)
	}
	t.subs = make(map[*subscriber]struct{})
}

type ttyWriteGate struct{ t *TTY }

func (g ttyWriteGate) Send(cmd string) error {
	return g.t.writeRawUnlocked(cmd + "\n")
}

func (g ttyWriteGate) SendRaw(s string) error {
	return g.t.writeRawUnlocked(s)
}

func (t *TTY) writeRawUnlocked(s string) error {
	if t == nil || t.master == nil {
		return io.ErrClosedPipe
	}
	_, err := t.master.Write([]byte(s))
	return err
}

// WithWrite holds the exclusive master write lock for the duration of fn.
func (t *TTY) WithWrite(ctx context.Context, fn func(w core.PTYWriter) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(ttyWriteGate{t})
}

func (t *TTY) Send(cmd string) error {
	return t.WithWrite(context.Background(), func(w core.PTYWriter) error {
		return w.Send(cmd)
	})
}

func (t *TTY) SendRaw(s string) error {
	return t.WithWrite(context.Background(), func(w core.PTYWriter) error {
		return w.SendRaw(s)
	})
}

// SignalInterrupt delivers SIGINT to the child process (Start mode only).
func (t *TTY) SignalInterrupt() error {
	if t == nil {
		return io.ErrClosedPipe
	}
	t.writeMu.Lock()
	var proc *os.Process
	if t.cmd != nil {
		proc = t.cmd.Process
	}
	t.writeMu.Unlock()
	if proc == nil {
		return io.ErrClosedPipe
	}
	return proc.Signal(os.Interrupt)
}

// SetSize updates the PTY window size.
func (t *TTY) SetSize(rows, cols uint16) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if t.master == nil {
		return io.ErrClosedPipe
	}
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}
	return pty.Setsize(t.master, &pty.Winsize{Rows: rows, Cols: cols})
}

// Pid returns the child process ID for Start mode, or 0.
func (t *TTY) Pid() int {
	if t == nil || t.cmd == nil || t.cmd.Process == nil {
		return 0
	}
	return t.cmd.Process.Pid
}

func (t *TTY) Close() {
	if t == nil {
		return
	}
	t.closeOnce.Do(func() {
		t.writeMu.Lock()
		if t.master != nil {
			_ = t.master.Close()
			t.master = nil
		}
		if t.slave != nil {
			_ = t.slave.Close()
			t.slave = nil
		}
		t.writeMu.Unlock()

		if t.cmd != nil && t.cmd.Process != nil {
			_ = t.cmd.Process.Kill()
			_, _ = t.cmd.Process.Wait()
			t.cmd = nil
		}

		t.subMu.Lock()
		t.closed = true
		for sub := range t.subs {
			sub.closeDone()
			close(sub.ch)
		}
		t.subs = nil
		t.subMu.Unlock()
	})
}
