package ptyx

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/creack/pty"
	"github.com/yairgd/gdbforge/internal/core"
)

// TTY is a bare PTY pair (no child process). gdbforge holds the master for
// read/write; the slave path is given to GDB via -inferior-tty-set so the
// debugged program's stdin/stdout attach there.
type TTY struct {
	master    *os.File
	slave     *os.File // kept open so the pts node stays valid
	slaveName string

	writeMu sync.Mutex

	subMu  sync.Mutex
	subs   map[chan core.PtyOutputMsg]struct{}
	closed bool

	closeOnce sync.Once
}

var _ core.Debugger = (*TTY)(nil)

// OpenTTY allocates a master/slave PTY for inferior I/O.
func OpenTTY() (*TTY, error) {
	master, slave, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("open inferior pty: %w", err)
	}
	name := slave.Name()
	if name == "" {
		_ = master.Close()
		_ = slave.Close()
		return nil, fmt.Errorf("inferior pty: empty slave name")
	}
	t := &TTY{
		master:    master,
		slave:     slave,
		slaveName: name,
		subs:      make(map[chan core.PtyOutputMsg]struct{}),
	}
	t.startReader()
	return t, nil
}

// SlaveName is the path passed to -inferior-tty-set (e.g. /dev/pts/5).
func (t *TTY) SlaveName() string {
	return t.slaveName
}

// Subscribe fans out raw bytes from the inferior (program stdout/stderr).
func (t *TTY) Subscribe() (<-chan core.PtyOutputMsg, func()) {
	ch := make(chan core.PtyOutputMsg, outputChanSize)

	t.subMu.Lock()
	if t.closed {
		t.subMu.Unlock()
		close(ch)
		return ch, func() {}
	}
	t.subs[ch] = struct{}{}
	t.subMu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			t.subMu.Lock()
			if _, ok := t.subs[ch]; ok {
				delete(t.subs, ch)
				close(ch)
			}
			t.subMu.Unlock()
		})
	}
	return ch, cancel
}

func (t *TTY) broadcast(msg core.PtyOutputMsg) {
	t.subMu.Lock()
	subs := make([]chan core.PtyOutputMsg, 0, len(t.subs))
	for ch := range t.subs {
		subs = append(subs, ch)
	}
	t.subMu.Unlock()
	for _, ch := range subs {
		// Blocking send applies backpressure: when the UI/coalescer is behind,
		// the master reader stalls, the kernel PTY buffer fills, and the
		// inferior's printf blocks — like a real terminal.
		// Recover if cancel closed the channel mid-send.
		func() {
			defer func() { _ = recover() }()
			ch <- msg
		}()
	}
}

func (t *TTY) startReader() {
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
	if t.closed {
		return
	}
	for ch := range t.subs {
		close(ch)
	}
	t.subs = make(map[chan core.PtyOutputMsg]struct{})
}

type ttyWriteGate struct{ t *TTY }

func (g ttyWriteGate) Send(cmd string) error {
	return g.t.writeRawUnlocked(cmd + "\n")
}

func (g ttyWriteGate) SendRaw(s string) error {
	return g.t.writeRawUnlocked(s)
}

func (t *TTY) writeRawUnlocked(s string) error {
	if t.master == nil {
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

// SetSize updates the inferior PTY window size.
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

func (t *TTY) Close() {
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

		t.subMu.Lock()
		t.closed = true
		for ch := range t.subs {
			close(ch)
		}
		t.subs = nil
		t.subMu.Unlock()
	})
}
