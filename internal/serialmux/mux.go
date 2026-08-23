package serialmux

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/yairgd/gdbforge/internal/devport"
	"golang.org/x/sys/unix"
)

var (
	ErrAlreadyOpen = errors.New("serialmux: device already open")
	ErrNotOpen     = errors.New("serialmux: not open")
)

// Owner selects which leg owns TX/RX on the shared UART (used after :debug-enter).
type Owner int

const (
	OwnerTerminal Owner = iota
	OwnerEnteringDebugger
	OwnerDebugger
)

func (o Owner) String() string {
	switch o {
	case OwnerTerminal:
		return "terminal"
	case OwnerEnteringDebugger:
		return "entering_debugger"
	case OwnerDebugger:
		return "debugger"
	default:
		return "unknown"
	}
}

// ParseOwner maps Lua/command strings to Owner.
func ParseOwner(s string) (Owner, error) {
	switch s {
	case "terminal", "":
		return OwnerTerminal, nil
	case "entering_debugger", "entering", "debug-enter":
		return OwnerEnteringDebugger, nil
	case "debugger", "gdb":
		return OwnerDebugger, nil
	default:
		return OwnerTerminal, fmt.Errorf("serialmux: unknown owner %q", s)
	}
}

type leg struct {
	master    *os.File
	slaveName string
}

func openLeg() (*leg, error) {
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
	_ = pty.Setsize(master, &pty.Winsize{Rows: 24, Cols: 80})
	if err := configurePTYRaw(master); err != nil {
		_ = master.Close()
		_ = slave.Close()
		return nil, err
	}
	// Hold master; xterm/minicom/GDB open slaveName (/dev/pts/N).
	_ = slave.Close()
	return &leg{master: master, slaveName: name}, nil
}

func configurePTYRaw(f *os.File) error {
	t, err := unix.IoctlGetTermios(int(f.Fd()), ioctlReadTermios)
	if err != nil {
		return fmt.Errorf("pty termios get: %w", err)
	}
	t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON | unix.IXOFF |
		unix.IXANY | unix.IMAXBEL
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ISIG | unix.ICANON | unix.ECHO | unix.ECHOE | unix.ECHOK |
		unix.ECHONL | unix.IEXTEN
	t.Cflag &^= unix.CSIZE | unix.PARENB | unix.CSTOPB
	t.Cflag |= unix.CS8 | unix.CREAD | unix.CLOCAL
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(f.Fd()), ioctlWriteTermios, t); err != nil {
		return fmt.Errorf("pty termios set: %w", err)
	}
	return nil
}

func (l *leg) close() {
	if l == nil || l.master == nil {
		return
	}
	_ = l.master.Close()
	l.master = nil
}

func writePTY(dst *os.File, b []byte) {
	if dst == nil || len(b) == 0 {
		return
	}
	_, _ = dst.Write(b)
}

// Mux bridges one physical UART and a published console PTY (/dev/pts/N).
type Mux struct {
	device string
	baud   int
	port   io.ReadWriteCloser

	gdbLeg  *leg
	termLeg *leg

	ownerMu sync.RWMutex
	owner   Owner
	miAuto  bool
	kgdbSerial bool // kgdb on shared UART: never auto-switch to console on ^running

	stop      chan struct{}
	wg        sync.WaitGroup
	portMu    sync.Mutex
	gdbMu     sync.Mutex
	gdbPumpOn bool

	closeOnce sync.Once
}

// Open claims device, publishes a console PTY, and starts USB <-> PTY pumps.
func Open(device string, baud int) (*Mux, error) {
	if err := claim(device); err != nil {
		return nil, err
	}
	port, err := devport.Open(device, baud)
	if err != nil {
		release(device)
		return nil, err
	}
	termLeg, err := openLeg()
	if err != nil {
		_ = port.Close()
		release(device)
		return nil, err
	}
	gdbLeg, err := openLeg()
	if err != nil {
		termLeg.close()
		_ = port.Close()
		release(device)
		return nil, err
	}
	m := &Mux{
		device:  device,
		baud:    baud,
		port:    port,
		termLeg: termLeg,
		gdbLeg:  gdbLeg,
		owner:   OwnerTerminal,
		stop:    make(chan struct{}),
	}
	register(device, m)
	m.start()
	return m, nil
}

func (m *Mux) start() {
	m.wg.Add(3)
	m.gdbPumpOn = true
	go m.pumpUSBToConsole()
	go m.pumpConsoleToUSB()
	go m.pumpGdbToUSB()
}

func (m *Mux) Device() string {
	if m == nil {
		return ""
	}
	return m.device
}

func (m *Mux) Baud() int {
	if m == nil {
		return 0
	}
	return m.baud
}

func (m *Mux) ensureGdbLeg() error {
	if m == nil {
		return ErrNotOpen
	}
	m.gdbMu.Lock()
	defer m.gdbMu.Unlock()
	if m.gdbLeg != nil {
		return nil
	}
	leg, err := openLeg()
	if err != nil {
		return err
	}
	m.gdbLeg = leg
	return nil
}

func (m *Mux) startGdbPump() {
	if m == nil {
		return
	}
	m.gdbMu.Lock()
	if m.gdbPumpOn {
		m.gdbMu.Unlock()
		return
	}
	m.gdbPumpOn = true
	m.gdbMu.Unlock()
	m.wg.Add(1)
	go m.pumpGdbToUSB()
}

func (m *Mux) DebuggerPTY() string {
	if m == nil {
		return ""
	}
	_ = m.ensureGdbLeg()
	if m.gdbLeg == nil {
		return ""
	}
	return m.gdbLeg.slaveName
}

func (m *Mux) TerminalPTY() string {
	if m == nil || m.termLeg == nil {
		return ""
	}
	return m.termLeg.slaveName
}

func (m *Mux) Owner() Owner {
	if m == nil {
		return OwnerTerminal
	}
	m.ownerMu.RLock()
	defer m.ownerMu.RUnlock()
	return m.owner
}

func (m *Mux) SetOwner(o Owner) {
	if m == nil {
		return
	}
	m.ownerMu.Lock()
	m.owner = o
	m.ownerMu.Unlock()
}

func (m *Mux) getOwner() Owner {
	m.ownerMu.RLock()
	defer m.ownerMu.RUnlock()
	return m.owner
}

func (m *Mux) routeSerialRx(b []byte) {
	if len(b) == 0 {
		return
	}
	owner := m.getOwner()
	switch owner {
	case OwnerEnteringDebugger:
		if m.gdbLeg != nil {
			writePTY(m.gdbLeg.master, b)
		}
		if m.termLeg != nil {
			writePTY(m.termLeg.master, b)
		}
	case OwnerDebugger:
		if m.gdbLeg != nil {
			writePTY(m.gdbLeg.master, b)
		}
	default:
		if m.termLeg != nil {
			writePTY(m.termLeg.master, b)
		}
	}
}

func (m *Mux) consoleTxAllowed() bool {
	owner := m.getOwner()
	return owner == OwnerTerminal || owner == OwnerEnteringDebugger
}

func (m *Mux) gdbTxAllowed() bool {
	owner := m.getOwner()
	return owner == OwnerEnteringDebugger || owner == OwnerDebugger
}

func (m *Mux) Send(line string) error {
	if m == nil || m.port == nil {
		return ErrNotOpen
	}
	if line == "" {
		return nil
	}
	if line[len(line)-1] != '\n' {
		line += "\n"
	}
	return m.writeSerial([]byte(line))
}

func (m *Mux) pumpUSBToConsole() {
	defer m.wg.Done()
	buf := make([]byte, 4096)
	for {
		select {
		case <-m.stop:
			return
		default:
		}
		n, err := m.port.Read(buf)
		if n > 0 {
			m.routeSerialRx(buf[:n])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
		}
		if n == 0 {
			select {
			case <-m.stop:
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
}

func (m *Mux) pumpConsoleToUSB() {
	defer m.wg.Done()
	if m.termLeg == nil || m.termLeg.master == nil {
		return
	}
	buf := make([]byte, 4096)
	for {
		select {
		case <-m.stop:
			return
		default:
		}
		rn, err := m.termLeg.master.Read(buf)
		if rn > 0 && m.consoleTxAllowed() {
			_ = m.writeSerial(buf[:rn])
		}
		if err != nil {
			select {
			case <-m.stop:
				return
			default:
			}
			if errors.Is(err, io.EOF) {
				continue
			}
			var errno syscall.Errno
			if errors.As(err, &errno) && (errno == syscall.EIO || errno == syscall.EBADF) {
				time.Sleep(20 * time.Millisecond)
				continue
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
	}
}

func (m *Mux) pumpGdbToUSB() {
	defer m.wg.Done()
	if m.gdbLeg == nil || m.gdbLeg.master == nil {
		return
	}
	buf := make([]byte, 4096)
	for {
		select {
		case <-m.stop:
			return
		default:
		}
		rn, err := m.gdbLeg.master.Read(buf)
		if rn > 0 && m.gdbTxAllowed() {
			_ = m.writeSerial(buf[:rn])
		}
		if err != nil {
			select {
			case <-m.stop:
				return
			default:
			}
			if errors.Is(err, io.EOF) {
				continue
			}
			var errno syscall.Errno
			if errors.As(err, &errno) && (errno == syscall.EIO || errno == syscall.EBADF) {
				time.Sleep(20 * time.Millisecond)
				continue
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
	}
}

func (m *Mux) writeSerial(b []byte) error {
	if m == nil || m.port == nil || len(b) == 0 {
		return nil
	}
	m.portMu.Lock()
	defer m.portMu.Unlock()
	for len(b) > 0 {
		n, err := m.port.Write(b)
		if n > 0 {
			b = b[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func (m *Mux) OnDebuggerState(stopped, promptReady, running bool) {
	if m == nil || !m.miAuto {
		return
	}
	switch {
	case running:
		if m.kgdbSerial {
			return // kgdb RSP owns UART until :serial-switch console
		}
		m.SetOwner(OwnerTerminal)
	case stopped:
		switch m.getOwner() {
		case OwnerEnteringDebugger, OwnerDebugger:
			m.SetOwner(OwnerDebugger)
		}
	case promptReady && m.getOwner() == OwnerEnteringDebugger:
		m.SetOwner(OwnerDebugger)
	}
}

func (m *Mux) BeginDebugEntry() {
	if m == nil {
		return
	}
	_ = m.ensureGdbLeg()
	m.startGdbPump()
	m.miAuto = true
	m.kgdbSerial = true
	m.SetOwner(OwnerEnteringDebugger)
}

// SwitchToGDB gives GDB exclusive use of the UART (console frozen on USB).
func (m *Mux) SwitchToGDB() {
	if m == nil {
		return
	}
	_ = m.ensureGdbLeg()
	m.startGdbPump()
	m.miAuto = true
	m.kgdbSerial = true
	m.SetOwner(OwnerDebugger)
}

// SwitchToConsole returns UART RX/TX to the console PTY.
func (m *Mux) SwitchToConsole() {
	if m == nil {
		return
	}
	m.miAuto = false
	m.kgdbSerial = false
	m.SetOwner(OwnerTerminal)
}

func (m *Mux) Close() error {
	if m == nil {
		return nil
	}
	var err error
	m.closeOnce.Do(func() {
		close(m.stop)
		if m.port != nil {
			err = m.port.Close()
		}
		if m.termLeg != nil {
			m.termLeg.close()
		}
		if m.gdbLeg != nil {
			m.gdbLeg.close()
		}
		release(m.device)
		// Pumps may still be draining; do not block the UI thread on quit (:q!).
		go m.wg.Wait()
	})
	return err
}
