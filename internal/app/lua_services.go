package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/termforge/service"
)

// Services is the debugger's service registry: the Lua gdbforge.* functions, the :AI tools
// and (the Remote ones) the -serve endpoint all call these. Built on first use.
func (a *DebuggerApp) Services() *service.Registry { return a.lua.services() }

func (c *luaCtl) services() *service.Registry {
	c.svcOnce.Do(func() {
		c.svc = mcp.Services(func() *mcp.GdbMcpService { return c.host.GdbMcp() })
		c.addServices(c.svc)
		c.addDebugServices(c.svc)
	})
	return c.svc
}

// onUI runs fn on the UI thread and waits. A call from Lua goes through callOnUI (which also
// honors job cancel); any other caller (REST/MCP goroutine) posts to the UI thread itself.
// A non-nil error means fn may not have run, so the caller must not read what fn sets.
func (c *luaCtl) onUI(ctx context.Context, fn func()) error {
	if luahost.FromLua(ctx) {
		c.callOnUI(fn)
		return nil
	}
	scr := c.host.Screen()
	if scr == nil {
		fn()
		return nil
	}
	done := make(chan struct{})
	if err := scr.PostEvent(tcell.NewEventInterrupt(luaUIMsg{fn: fn, done: done})); err != nil {
		return err
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// addServices registers the old Lua gdbforge.* functions once as services. Remote ones are
// read-only queries; everything that changes state or touches the serial line stays Lua-only.
func (c *luaCtl) addServices(r *service.Registry) {
	h := c.host
	str := func(n, help string) service.Param { return service.Param{Name: n, Type: service.String, Help: help} }
	opt := func(p service.Param) service.Param { p.Optional = true; return p }
	num := func(n, help string) service.Param { return service.Param{Name: n, Type: service.Number, Help: help} }

	// query runs a read on the UI thread and returns what it set.
	query := func(read func() any) func(context.Context, service.Args) (any, error) {
		return func(ctx context.Context, _ service.Args) (any, error) {
			var v any
			if err := c.onUI(ctx, func() { v = read() }); err != nil {
				return nil, err
			}
			return v, nil
		}
	}
	// serial runs fn off the UI thread: serial ops use the serialCtl mutex; going through the UI
	// would wedge the caller while the UI processes GDB MI (see :lua kgdb_serial).
	serial := func(fn func(a service.Args) (any, error)) func(context.Context, service.Args) (any, error) {
		return func(_ context.Context, a service.Args) (any, error) { return fn(a) }
	}

	for _, s := range []service.Service{
		{Name: "program", Remote: true, Help: "program path of the session",
			Run: func(context.Context, service.Args) (any, error) { return h.SessionProgram(), nil }},
		{Name: "debugger_path", Remote: true, Help: "path of the debugger binary",
			Run: func(context.Context, service.Args) (any, error) { return h.DebuggerPath(), nil }},
		{Name: "current_file", Remote: true, Help: "file of the active code pane, else of the current frame",
			Run: query(func() any {
				path := ""
				if cw := h.ActiveCodeWidget(); cw != nil {
					path = cw.Path()
				}
				if path == "" && h.Debug() != nil {
					path = h.Debug().CurrentFile()
				}
				return path
			})},
		{Name: "current_line", Remote: true, Help: "cursor line of the active code pane, else of the current frame",
			Run: query(func() any {
				line := 0
				if cw := h.ActiveCodeWidget(); cw != nil {
					line = cw.SelLine()
				}
				if line < 1 && h.Debug() != nil {
					line = h.Debug().CurrentLine()
				}
				return line
			})},
		{Name: "stop_file", Remote: true, Help: "file where the program last stopped",
			Run: query(func() any {
				path := ""
				if h.Debug() != nil {
					path = h.Debug().StopFile()
				}
				if path == "" {
					if cw := h.ActiveCodeWidget(); cw != nil && cw.PCLine() > 0 {
						path = cw.Path()
					}
				}
				return path
			})},
		{Name: "stop_line", Remote: true, Help: "line where the program last stopped",
			Run: query(func() any {
				line := 0
				if h.Debug() != nil {
					line = h.Debug().StopLine()
				}
				if line < 1 {
					if cw := h.ActiveCodeWidget(); cw != nil {
						line = cw.PCLine()
					}
				}
				return line
			})},
		{Name: "gdb", Help: "send a command to the debugger as if typed in the gdb pane", Params: []service.Param{str("cmd", "command line")},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				cmd := strings.TrimSpace(a.String("cmd"))
				if cmd == "" {
					return nil, nil
				}
				return nil, c.onUI(ctx, func() {
					if h.GDBWidget() != nil {
						h.GDBWidget().AppendHostLine(">>> " + cmd)
					}
					h.SendGdbExec(cmd)
					h.RequestFrame()
				})
			}},
		{Name: "set_inferior_tty", Help: "route program stdio to a terminal (pts path or \"external\")", Params: []service.Param{str("path", "pts path or external")},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				path := strings.TrimSpace(a.String("path"))
				var err error
				if e := c.onUI(ctx, func() { err = h.SetInferiorTTY(path) }); e != nil {
					return nil, e
				}
				if err != nil {
					return nil, err
				}
				return "inferior tty: " + path, nil
			}},
		{Name: "dlv_connect", Help: "connect to a headless Delve at addr", Params: []service.Param{str("addr", "host:port")},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				addr := strings.TrimSpace(a.String("addr"))
				var err error
				if e := c.onUI(ctx, func() { err = h.ConnectDlv(addr) }); e != nil {
					return nil, e
				}
				if err != nil {
					return nil, err
				}
				return "dlv_connect: " + addr, nil
			}},
		{Name: "spawn_dlv_headless", Help: "start a headless Delve on port (default 2345); extra args follow --",
			Params: []service.Param{opt(str("port", "port, default 2345")), {Name: "args", Type: service.String, Help: "extra dlv arguments", Variadic: true}},
			Run: func(_ context.Context, a service.Args) (any, error) {
				port := strings.TrimSpace(a.String("port"))
				if port == "" {
					port = "2345"
				}
				var extra []string
				for _, s := range a.Strings("args") {
					if s = strings.TrimSpace(s); s != "" {
						extra = append(extra, s)
					}
				}
				if err := h.SpawnDlvHeadless(port, extra); err != nil {
					return nil, err
				}
				msg := "spawn_dlv_headless :" + port
				if len(extra) > 0 {
					msg += " -- " + strings.Join(extra, " ")
				}
				return msg, nil
			}},
		{Name: "gdb_query", Help: "run a debugger command and return its output (timeout in seconds, default 120)",
			Params: []service.Param{str("cmd", "command line"), opt(num("timeout", "seconds"))},
			Run: func(ctx context.Context, a service.Args) (any, error) {
				cmd := strings.TrimSpace(a.String("cmd"))
				if cmd == "" {
					return nil, fmt.Errorf("empty gdb command")
				}
				m := h.GdbMcp()
				if m == nil {
					return nil, fmt.Errorf("gdb_query: no gdb session")
				}
				c.echoGdbWidget(h, cmd)
				timeout := 120.0
				if t, ok := a["timeout"].(float64); ok && t > 0 {
					timeout = t
				}
				ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
				defer cancel()
				if gdb.IsTargetRemoteCmd(cmd) {
					return m.QueryLong(ctx, cmd)
				}
				return m.Query(ctx, cmd)
			}},
		{Name: "gdb_ctrl_c", Help: "interrupt the running program (^C)",
			Run: func(context.Context, service.Args) (any, error) {
				c.echoGdbWidget(h, "^C")
				if b := h.Backend(); b != nil {
					running := h.Debug() != nil && h.Debug().InferiorRunning()
					_ = b.Interrupt(running, false)
				}
				h.RequestFrame()
				return nil, nil
			}},
		{Name: "set_kgdb_mode", Help: "switch kgdb mode on (default) or off", Params: []service.Param{opt(service.Param{Name: "on", Type: service.Boolean, Help: "default true"})},
			Run: func(_ context.Context, a service.Args) (any, error) {
				on := true
				if v, ok := a["on"].(bool); ok {
					on = v
				}
				h.SetKgdbMode(on)
				return nil, nil
			}},
		{Name: "uart_send", Help: "write a line to a serial device (baud default 115200)",
			Params: []service.Param{str("device", "serial device"), str("line", "text, CRLF is appended"), opt(service.Param{Name: "baud", Type: service.Integer, Help: "default 115200"})},
			Run: func(_ context.Context, a service.Args) (any, error) {
				baud := 115200
				if b := a.Int("baud"); b > 0 {
					baud = b
				}
				return nil, sendUartLine(a.String("device"), baud, a.String("line"))
			}},
		{Name: "open_serial_terminal", Help: "open the shared serial terminal on device (baud default 115200)",
			Params: []service.Param{str("device", "serial device"), opt(service.Param{Name: "baud", Type: service.Integer, Help: "default 115200"})},
			Run: serial(func(a service.Args) (any, error) {
				baud := 115200
				if b := a.Int("baud"); b > 0 {
					baud = b
				}
				device := strings.TrimSpace(a.String("device"))
				if err := h.Serial().OpenShared(device, baud); err != nil {
					return nil, err
				}
				return fmt.Sprintf("serial: %s @ %d", device, baud), nil
			})},
		{Name: "serial_debugger_pty", Help: "pty path the debugger uses for the serial line",
			Run: serial(func(service.Args) (any, error) { return h.Serial().DebuggerPTY() })},
		{Name: "serial_terminal_pty", Help: "pty path of the serial terminal",
			Run: serial(func(service.Args) (any, error) { return h.Serial().TerminalPTY() })},
		{Name: "serial_send", Help: "send a line on the serial line", Params: []service.Param{str("line", "text")},
			Run: serial(func(a service.Args) (any, error) { return nil, h.Serial().Send(a.String("line")) })},
		{Name: "serial_switch_owner", Help: "give the serial line to a mode", Params: []service.Param{str("mode", "owner mode")},
			Run: serial(func(a service.Args) (any, error) {
				return nil, h.Serial().SwitchOwner(strings.TrimSpace(a.String("mode")))
			})},
		{Name: "serial_owner", Help: "current owner of the serial line",
			Run: serial(func(service.Args) (any, error) { return h.Serial().Owner() })},
		{Name: "begin_debug_entry", Help: "start the debug entry sequence on the serial line",
			Run: serial(func(service.Args) (any, error) { return nil, h.Serial().BeginDebugEntry() })},
		{Name: "serial_switch_gdb", Help: "hand the serial line to the debugger (kgdb mode on)",
			Run: serial(func(service.Args) (any, error) {
				if err := h.Serial().SwitchToGDB(); err != nil {
					return nil, err
				}
				h.SetKgdbMode(true)
				return nil, nil
			})},
		{Name: "serial_switch_console", Help: "hand the serial line back to the console",
			Run: serial(func(service.Args) (any, error) { return nil, h.Serial().SwitchToConsole() })},
		{Name: "serial_sysrq_delayed", Help: "send SysRq-g after a delay in seconds (default 2)", Params: []service.Param{opt(num("delay", "seconds"))},
			Run: serial(func(a service.Args) (any, error) {
				delay := 2.0
				if d, ok := a["delay"].(float64); ok {
					delay = d
				}
				return nil, h.Serial().SysrqDelayed(delay)
			})},
		{Name: "close_serial_terminal", Help: "close the serial terminal",
			Run: serial(func(service.Args) (any, error) { h.Serial().Close(); return nil, nil })},
		{Name: "spawn_serial_console", Help: "run a serial console on a pty (baud default 115200)",
			Params: []service.Param{str("pty", "pty path"), opt(service.Param{Name: "baud", Type: service.Integer, Help: "default 115200"})},
			Run: serial(func(a service.Args) (any, error) {
				baud := 115200
				if b := a.Int("baud"); b > 0 {
					baud = b
				}
				return nil, h.Serial().SpawnConsole(strings.TrimSpace(a.String("pty")), baud)
			})},
	} {
		r.Add(s)
	}
}

// luaPrelude keeps the call shapes scripts already use on top of the services:
// gdb_query returns (out, err) instead of raising, and uart_send accepts (dev, baud, line).
const luaPrelude = `
do
  local q = gdbforge.gdb_query
  gdbforge.gdb_query = function(cmd, timeout)
    local ok, out = pcall(q, cmd, timeout)
    if ok then return out, "" end
    return "", (tostring(out):gsub("^[^:\n]*:%d+: ", ""))
  end
  local u = gdbforge.uart_send
  gdbforge.uart_send = function(device, a, b)
    if b ~= nil then return u(device, b, tonumber(a)) end
    return u(device, a)
  end
end
`
