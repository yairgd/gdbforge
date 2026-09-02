package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/ptyx"
	"github.com/yairgd/gdbforge/internal/serialmux"
)

// serialCtl owns the shared UART mux (one device per gdbforge session).
type serialCtl struct {
	app *DebuggerApp
	mu  sync.Mutex
	mux *serialmux.Mux
	// switchConsoleOnRunning: arm on (gdb) continue; switch after ^running so
	// GDB can finish the continue packet on the gdb PTY first.
	switchConsoleOnRunning bool
}

func (c *serialCtl) Active() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mux != nil
}

func (c *serialCtl) Device() string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mux == nil {
		return ""
	}
	return c.mux.Device()
}

func (c *serialCtl) Baud() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mux == nil {
		return 0
	}
	return c.mux.Baud()
}

func (c *serialCtl) OpenShared(device string, baud int) error {
	if c == nil {
		return fmt.Errorf("serial: no app")
	}
	device = strings.TrimSpace(device)
	if device == "" {
		return fmt.Errorf("serial: empty device")
	}
	if baud <= 0 {
		baud = 115200
	}
	if err := reclaimUART(device); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mux != nil {
		if c.mux.Device() == device {
			return nil
		}
		return fmt.Errorf("serial: already open on %s", c.mux.Device())
	}
	m, err := serialmux.Open(device, baud)
	if err != nil {
		return err
	}
	c.mux = m
	return nil
}

func (c *serialCtl) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	m := c.mux
	c.mux = nil
	c.mu.Unlock()
	if m != nil {
		_ = m.Close()
	}
}

func (c *serialCtl) muxOrErr() (*serialmux.Mux, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mux == nil {
		return nil, fmt.Errorf("serial: not open — use gdbforge.open_serial_terminal")
	}
	return c.mux, nil
}

func (c *serialCtl) DebuggerPTY() (string, error) {
	m, err := c.muxOrErr()
	if err != nil {
		return "", err
	}
	return m.DebuggerPTY(), nil
}

func (c *serialCtl) TerminalPTY() (string, error) {
	m, err := c.muxOrErr()
	if err != nil {
		return "", err
	}
	return m.TerminalPTY(), nil
}

func (c *serialCtl) TermTTY() (*ptyx.TTY, error) {
	m, err := c.muxOrErr()
	if err != nil {
		return nil, err
	}
	return m.TermTTY(), nil
}

func (c *serialCtl) Send(line string) error {
	m, err := c.muxOrErr()
	if err != nil {
		return err
	}
	return m.Send(line)
}

func (c *serialCtl) SwitchOwner(mode string) error {
	m, err := c.muxOrErr()
	if err != nil {
		return err
	}
	o, err := serialmux.ParseOwner(mode)
	if err != nil {
		return err
	}
	m.SetOwner(o)
	return nil
}

func (c *serialCtl) Owner() (string, error) {
	m, err := c.muxOrErr()
	if err != nil {
		return "", err
	}
	return m.Owner().String(), nil
}

func (c *serialCtl) SysrqDelayed(delaySec float64) error {
	m, err := c.muxOrErr()
	if err != nil {
		return err
	}
	if delaySec < 0 {
		delaySec = 0
	}
	go func() {
		time.Sleep(time.Duration(delaySec * float64(time.Second)))
		_ = m.Send("echo g > /proc/sysrq-trigger")
	}()
	return nil
}

func (c *serialCtl) BeginDebugEntry() error {
	m, err := c.muxOrErr()
	if err != nil {
		return err
	}
	m.BeginDebugEntry()
	return nil
}

func (c *serialCtl) SwitchToGDB() error {
	m, err := c.muxOrErr()
	if err != nil {
		return err
	}
	m.SwitchToGDB()
	return nil
}

func (c *serialCtl) SwitchToConsole() error {
	m, err := c.muxOrErr()
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.switchConsoleOnRunning = false
	c.mu.Unlock()
	m.SwitchToConsole()
	return nil
}

func (c *serialCtl) ArmSwitchConsoleOnRunning() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.switchConsoleOnRunning = true
	c.mu.Unlock()
}

func (c *serialCtl) takeSwitchConsoleOnRunning() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	v := c.switchConsoleOnRunning
	c.switchConsoleOnRunning = false
	c.mu.Unlock()
	return v
}

func (c *serialCtl) OnDebuggerState(stopped, promptReady, running bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	m := c.mux
	c.mu.Unlock()
	if m == nil {
		return
	}
	m.OnDebuggerState(stopped, promptReady, running)
}

func (a *DebuggerApp) OnDebugEnter(_ ...any) {
	if err := a.serial.BeginDebugEntry(); err != nil {
		a.printHostLine("debug-enter: " + err.Error())
		return
	}
	gdb, _ := a.serial.DebuggerPTY()
	a.printHostLine("debug-enter: UART shared (gdb+console RX) — use before kgdb break-in")
	if gdb != "" {
		a.printHostLine("  gdb PTY: " + gdb)
	}
}

// OnSerialSwitchCmd switches UART routing between console and GDB legs.
//
//	:serial-switch gdb       — GDB owns USB (before/after target remote)
//	:serial-switch console   — console/minicom owns USB
//	:serial-switch status
func (a *DebuggerApp) OnSerialSwitchCmd(args ...any) {
	tokens := cmdArgs(args)
	if len(tokens) == 0 {
		a.serialSwitchStatus()
		return
	}
	switch strings.ToLower(tokens[0]) {
	case "gdb", "debugger", "debug":
		if err := a.serial.SwitchToGDB(); err != nil {
			a.printHostLine("serial-switch: " + err.Error())
			return
		}
		a.setKgdbMode(true)
		gdb, _ := a.serial.DebuggerPTY()
		a.printHostLine("serial-switch: gdb — USB RX/TX on gdb PTY only")
		if gdb != "" {
			a.printHostLine("  (gdb) target remote " + gdb)
		}
	case "console", "terminal", "term":
		if err := a.serial.SwitchToConsole(); err != nil {
			a.printHostLine("serial-switch: " + err.Error())
			return
		}
		pty, _ := a.serial.TerminalPTY()
		a.printHostLine("serial-switch: console — USB RX/TX on console PTY (IO pane)")
		if pty != "" {
			a.printHostLine("  console PTY: " + pty)
		}
		if err := a.wireSerialConsole(); err != nil {
			a.printHostLine("serial-switch: wire IO: " + err.Error())
		}
	case "status":
		a.serialSwitchStatus()
	default:
		a.printHostLine("serial-switch: use gdb | console | status")
	}
}

func (a *DebuggerApp) serialSwitchStatus() {
	if !a.serial.Active() {
		a.printHostLine("serial-switch: serial mux not open")
		return
	}
	owner, _ := a.serial.Owner()
	con, _ := a.serial.TerminalPTY()
	gdb, _ := a.serial.DebuggerPTY()
	a.printHostLine("serial-switch: owner=" + owner)
	if con != "" {
		a.printHostLine("  console PTY: " + con)
	}
	if gdb != "" {
		a.printHostLine("  gdb PTY:     " + gdb)
	}
}

func (a *DebuggerApp) printHostLine(line string) {
	if a.outputWidget != nil {
		a.outputWidget.AppendHostLine(line)
		a.RequestFrame()
	}
}

// OnTerminalCmd opens a USB <-> PTY console bridge (stage-1 manual test, no Lua).
//
//	:terminal                      — /dev/ttyUSB0 @ 115200
//	:terminal /dev/ttyUSB0 [baud]
//	:terminal status
//	:terminal close
func (a *DebuggerApp) OnTerminalCmd(args ...any) {
	tokens := cmdArgs(args)
	if len(tokens) == 0 {
		a.terminalOpen("", 0)
		return
	}
	switch strings.ToLower(tokens[0]) {
	case "close", "stop":
		a.serial.Close()
		a.restoreInferiorIO()
		a.printHostLine("terminal: closed")
	case "status":
		a.terminalStatus()
	default:
		device := tokens[0]
		baud := 115200
		if len(tokens) >= 2 {
			if n, err := strconv.Atoi(tokens[1]); err == nil && n > 0 {
				baud = n
			}
		}
		a.terminalOpen(device, baud)
	}
}

func cmdArgs(args []any) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if s, ok := a.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func (a *DebuggerApp) terminalStatus() {
	if !a.serial.Active() {
		a.printHostLine("terminal: not open")
		return
	}
	pty, err := a.serial.TerminalPTY()
	if err != nil {
		a.printHostLine("terminal: " + err.Error())
		return
	}
	gdb, _ := a.serial.DebuggerPTY()
	a.printHostLine(fmt.Sprintf("terminal: %s @ %d", a.serial.Device(), a.serial.Baud()))
	a.printHostLine("  console PTY: " + pty)
	if gdb != "" {
		a.printHostLine("  gdb PTY:     " + gdb + "  (reserved)")
	}
	a.printHostLine("  gdbforge holds USB; serial console uses IO pane")
}

func (a *DebuggerApp) terminalOpen(device string, baud int) {
	device = strings.TrimSpace(device)
	if device == "" {
		device = strings.TrimSpace(os.Getenv("GDBFORGE_KGDB_UART"))
		if device == "" {
			device = "/dev/ttyUSB0"
		}
	}
	if baud <= 0 {
		baud = 115200
		if v := strings.TrimSpace(os.Getenv("GDBFORGE_KGDB_BAUD")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				baud = n
			}
		}
	}

	if err := a.serial.OpenShared(device, baud); err != nil {
		a.printHostLine("terminal: " + err.Error())
		return
	}
	pty, err := a.serial.TerminalPTY()
	if err != nil {
		a.printHostLine("terminal: " + err.Error())
		return
	}

	useExternal := strings.EqualFold(strings.TrimSpace(os.Getenv("GDBFORGE_EXTERNAL_SERIAL")), "1")
	if useExternal {
		if err := spawnConsoleTerminal(a, pty, baud); err != nil {
			a.printHostLine("terminal: spawn: " + err.Error())
			return
		}
	} else if err := a.wireSerialConsole(); err != nil {
		a.printHostLine("terminal: wire IO: " + err.Error())
		return
	}

	gdb, _ := a.serial.DebuggerPTY()
	a.printHostLine(fmt.Sprintf("terminal: %s @ %d", device, baud))
	a.printHostLine("  console PTY: " + pty)
	if gdb != "" {
		a.printHostLine("  gdb PTY:     " + gdb + "  (reserved)")
	}
	if useExternal {
		a.printHostLine("  external minicom/screen (GDBFORGE_EXTERNAL_SERIAL=1)")
	} else {
		a.printHostLine("  serial console on IO pane (set GDBFORGE_EXTERNAL_SERIAL=1 for minicom)")
	}
	a.printHostLine("  :terminal close  — release port")
}

func reclaimUART(device string) error {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GDBFORGE_KGDB_TAKEOVER")))
	if mode == "" {
		mode = "auto"
	}
	if mode == "never" {
		return nil
	}
	pids := fuserPIDs(device)
	if len(pids) == 0 {
		return nil
	}
	if mode != "auto" && mode != "force" {
		return fmt.Errorf("%s busy (set GDBFORGE_KGDB_TAKEOVER=force)", device)
	}
	for _, pid := range pids {
		_ = exec.Command("kill", pid).Run()
	}
	time.Sleep(300 * time.Millisecond)
	if remain := fuserPIDs(device); len(remain) > 0 {
		for _, pid := range remain {
			_ = exec.Command("kill", "-9", pid).Run()
		}
		time.Sleep(200 * time.Millisecond)
	}
	if remain := fuserPIDs(device); len(remain) > 0 {
		return fmt.Errorf("%s still busy after reclaim (fuser -k %s)", device, device)
	}
	return nil
}

func fuserPIDs(device string) []string {
	out, err := exec.Command("fuser", device).CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var pids []string
	for _, field := range strings.Fields(string(out)) {
		for _, pid := range strings.Split(field, ":") {
			pid = strings.TrimSpace(pid)
			if pid != "" && !seen[pid] {
				seen[pid] = true
				pids = append(pids, pid)
			}
		}
	}
	return pids
}

func (c *luaCtl) installSerialAPI(rt *luahost.Runtime) {
	if rt == nil {
		return
	}
	h := c.host
	// Serial ops use serialCtl mutex; do not callOnUI (worker would block while UI
	// processes GDB MI, wedging :lua kgdb_serial after minicom closes).
	rt.SetGdbforgeFunc("open_serial_terminal", func(L *lua.LState) int {
		device := strings.TrimSpace(L.CheckString(1))
		baud := 115200
		if L.GetTop() >= 2 {
			baud = int(L.CheckNumber(2))
		}
		if err := h.Serial().OpenShared(device, baud); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		rt.AppendPrint("serial: " + device + " @ " + strconv.Itoa(baud))
		return 0
	})
	rt.SetGdbforgeFunc("serial_debugger_pty", func(L *lua.LState) int {
		path, err := h.Serial().DebuggerPTY()
		if err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		L.Push(lua.LString(path))
		return 1
	})
	rt.SetGdbforgeFunc("serial_terminal_pty", func(L *lua.LState) int {
		path, err := h.Serial().TerminalPTY()
		if err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		L.Push(lua.LString(path))
		return 1
	})
	rt.SetGdbforgeFunc("serial_send", func(L *lua.LState) int {
		line := L.CheckString(1)
		if err := h.Serial().Send(line); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		return 0
	})
	rt.SetGdbforgeFunc("serial_switch_owner", func(L *lua.LState) int {
		mode := strings.TrimSpace(L.CheckString(1))
		if err := h.Serial().SwitchOwner(mode); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		return 0
	})
	rt.SetGdbforgeFunc("serial_owner", func(L *lua.LState) int {
		owner, err := h.Serial().Owner()
		if err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		L.Push(lua.LString(owner))
		return 1
	})
	rt.SetGdbforgeFunc("begin_debug_entry", func(L *lua.LState) int {
		if err := h.Serial().BeginDebugEntry(); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		return 0
	})
	rt.SetGdbforgeFunc("serial_switch_gdb", func(L *lua.LState) int {
		if err := h.Serial().SwitchToGDB(); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		h.SetKgdbMode(true)
		return 0
	})
	rt.SetGdbforgeFunc("serial_switch_console", func(L *lua.LState) int {
		if err := h.Serial().SwitchToConsole(); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		return 0
	})
	rt.SetGdbforgeFunc("serial_sysrq_delayed", func(L *lua.LState) int {
		delay := 2.0
		if L.GetTop() >= 1 {
			delay = float64(L.CheckNumber(1))
		}
		if err := h.Serial().SysrqDelayed(delay); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		return 0
	})
	rt.SetGdbforgeFunc("close_serial_terminal", func(L *lua.LState) int {
		h.Serial().Close()
		return 0
	})
	rt.SetGdbforgeFunc("spawn_serial_console", func(L *lua.LState) int {
		pty := strings.TrimSpace(L.CheckString(1))
		baud := 115200
		if L.GetTop() >= 2 {
			baud = int(L.CheckNumber(2))
		}
		if err := h.Serial().SpawnConsole(pty, baud); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		return 0
	})
}
