package pydbg

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// errClosed is returned by calls on a connection whose stream ended.
var errClosed = errors.New("dap connection closed")

// dapMessage is the union of DAP request, response and event envelopes.
type dapMessage struct {
	Seq        int             `json:"seq"`
	Type       string          `json:"type"`
	Command    string          `json:"command,omitempty"`
	Arguments  any             `json:"arguments,omitempty"`
	RequestSeq int             `json:"request_seq,omitempty"`
	Success    bool            `json:"success"`
	Message    string          `json:"message,omitempty"`
	Event      string          `json:"event,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
}

// dapEvent is one adapter event handed to the handler.
type dapEvent struct {
	name string
	body json.RawMessage
}

// dapConn is a DAP client over a byte stream. Requests are matched to
// responses by seq; events go to the handler on a goroutine of their own, so
// the handler may issue requests.
type dapConn struct {
	rwc io.ReadWriteCloser
	br  *bufio.Reader

	wmu sync.Mutex // serializes writes

	mu      sync.Mutex
	seq     int
	pending map[int]chan dapMessage

	events    chan dapEvent
	done      chan struct{}
	closeOnce sync.Once
}

// dapCall is a request in flight.
type dapCall struct {
	conn    *dapConn
	seq     int
	command string
	ch      chan dapMessage
}

// writeDAP writes one framed message.
func writeDAP(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(data), data)
	return err
}

// readDAP reads one framed message body.
func readDAP(br *bufio.Reader) ([]byte, error) {
	n := -1
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if n < 0 {
				return nil, fmt.Errorf("dap: missing Content-Length")
			}
			break
		}
		if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(k, "Content-Length") {
			if n, err = strconv.Atoi(strings.TrimSpace(v)); err != nil || n < 0 {
				return nil, fmt.Errorf("dap: bad Content-Length %q", v)
			}
		}
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(br, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// newDapConn starts the reader and event goroutines. onEvent may be nil.
func newDapConn(rwc io.ReadWriteCloser, onEvent func(name string, body json.RawMessage)) *dapConn {
	d := &dapConn{
		rwc:     rwc,
		br:      bufio.NewReader(rwc),
		pending: make(map[int]chan dapMessage),
		events:  make(chan dapEvent, 256),
		done:    make(chan struct{}),
	}
	go d.readLoop()
	go func() {
		for {
			select {
			case ev := <-d.events:
				if onEvent != nil {
					onEvent(ev.name, ev.body)
				}
			case <-d.done:
				return
			}
		}
	}()
	return d
}

func (d *dapConn) readLoop() {
	defer d.close()
	for {
		data, err := readDAP(d.br)
		if err != nil {
			return
		}
		var m dapMessage
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.Type {
		case "response":
			d.mu.Lock()
			ch := d.pending[m.RequestSeq]
			delete(d.pending, m.RequestSeq)
			d.mu.Unlock()
			if ch != nil {
				ch <- m
			}
		case "event":
			select {
			case d.events <- dapEvent{m.Event, m.Body}:
			case <-d.done:
				return
			}
		case "request":
			// Reverse requests (runInTerminal, ...) are not supported.
			_ = d.write(dapMessage{Type: "response", RequestSeq: m.Seq, Command: m.Command,
				Success: false, Message: "unsupported"})
		}
	}
}

func (d *dapConn) write(m dapMessage) error {
	d.wmu.Lock()
	defer d.wmu.Unlock()
	d.mu.Lock()
	d.seq++
	m.Seq = d.seq
	d.mu.Unlock()
	return writeDAP(d.rwc, m)
}

// close ends the connection; pending calls fail with errClosed.
func (d *dapConn) close() {
	d.closeOnce.Do(func() {
		close(d.done)
		_ = d.rwc.Close()
	})
}

// start sends a request without waiting for the response.
func (d *dapConn) start(command string, args any) (*dapCall, error) {
	ch := make(chan dapMessage, 1)
	d.wmu.Lock()
	d.mu.Lock()
	d.seq++
	seq := d.seq
	d.pending[seq] = ch
	d.mu.Unlock()
	err := writeDAP(d.rwc, dapMessage{Seq: seq, Type: "request", Command: command, Arguments: args})
	d.wmu.Unlock()
	if err != nil {
		d.mu.Lock()
		delete(d.pending, seq)
		d.mu.Unlock()
		return nil, fmt.Errorf("dap %s: %w", command, err)
	}
	return &dapCall{conn: d, seq: seq, command: command, ch: ch}, nil
}

// wait returns the response body, or the adapter's error message.
func (c *dapCall) wait(timeout time.Duration) (json.RawMessage, error) {
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case m := <-c.ch:
		if !m.Success {
			msg := m.Message
			var b struct {
				Error struct{ Format string } `json:"error"`
			}
			if json.Unmarshal(m.Body, &b) == nil && b.Error.Format != "" {
				msg = b.Error.Format
			}
			return nil, fmt.Errorf("dap %s: %s", c.command, msg)
		}
		return m.Body, nil
	case <-c.conn.done:
		return nil, fmt.Errorf("dap %s: %w", c.command, errClosed)
	case <-t.C:
		c.conn.mu.Lock()
		delete(c.conn.pending, c.seq)
		c.conn.mu.Unlock()
		return nil, fmt.Errorf("dap %s: timeout after %s", c.command, timeout)
	}
}

// call sends a request and waits for its response.
func (d *dapConn) call(command string, args any, timeout time.Duration) (json.RawMessage, error) {
	c, err := d.start(command, args)
	if err != nil {
		return nil, err
	}
	return c.wait(timeout)
}
