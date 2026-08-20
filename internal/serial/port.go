package serial

import (
	"errors"
	"io"
	"sync"
	"time"
)

var (
	ErrClosed  = errors.New("serial: closed")
	ErrNotOpen = errors.New("serial: not open")
)

// Port is an exclusive raw serial device (one reader, serialized writers).
type Port struct {
	mu     sync.RWMutex
	closed bool
	rw     io.ReadWriteCloser
	name   string
}

// DeviceName returns the path passed to Open (e.g. /dev/ttyUSB0).
func (p *Port) DeviceName() string {
	if p == nil {
		return ""
	}
	return p.name
}

func (p *Port) Read(b []byte) (int, error) {
	p.mu.RLock()
	if p.closed || p.rw == nil {
		p.mu.RUnlock()
		return 0, ErrClosed
	}
	rw := p.rw
	p.mu.RUnlock()
	return rw.Read(b)
}

func (p *Port) Write(b []byte) (int, error) {
	p.mu.RLock()
	if p.closed || p.rw == nil {
		p.mu.RUnlock()
		return 0, ErrClosed
	}
	rw := p.rw
	p.mu.RUnlock()
	return rw.Write(b)
}

// SetReadDeadline forwards to the underlying device when supported (*os.File).
func (p *Port) SetReadDeadline(t time.Time) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed || p.rw == nil {
		return ErrClosed
	}
	if d, ok := p.rw.(interface{ SetReadDeadline(time.Time) error }); ok {
		return d.SetReadDeadline(t)
	}
	return nil
}

// Close releases the device.
func (p *Port) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	if p.rw == nil {
		return nil
	}
	err := p.rw.Close()
	p.rw = nil
	return err
}
