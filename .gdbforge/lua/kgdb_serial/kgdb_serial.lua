-- Serial mux: gdbforge holds /dev/ttyUSB0, publishes two PTYs.
--
-- Usage: :lua kgdb_serial
--
-- Automated break-in (recommended):
--   :lua kgdb_serial              once — open mux + minicom
--   :lua kgdb_trigger             each break-in (same order as manual below)
--   (gdb) n/s/c …                 continue returns UART to minicom on ^running
--   :lua kgdb_trigger console     manual UART return (optional)
--
-- Manual attach (validated order — switch gdb BEFORE sysrq):
--   :serial-switch gdb
--   (gdb) target remote <gdb PTY>   ← NOT the console PTY
--   gdbforge.serial_send("echo g > /proc/sysrq-trigger")
--   :serial-switch console          when done (or gdb continue auto-switches)

local function env(name, fallback)
  local v = os.getenv(name)
  if v == nil or v:match("^%s*$") then
    return fallback
  end
  return v:match("^%s*(.-)%s*$")
end

function help()
  gdbforge.print("kgdb_serial — USB mux + console + gdb PTY")
  gdbforge.print("Usage: :lua kgdb_serial")
  gdbforge.print("")
  gdbforge.print("Recommended workflow:")
  gdbforge.print("  :lua kgdb_serial           once per session")
  gdbforge.print("  :lua kgdb_trigger          break in each time")
  gdbforge.print("  (gdb) continue             auto back to minicom")
  gdbforge.print("")
  gdbforge.print("Manual (validated order):")
  gdbforge.print("  :serial-switch gdb")
  gdbforge.print("  (gdb) target remote <gdb PTY>   -- not the console PTY")
  gdbforge.print("  gdbforge.serial_send('echo g > /proc/sysrq-trigger')")
  gdbforge.print("  (gdb) continue   or  :serial-switch console")
end

function main()
  local uart = env("GDBFORGE_KGDB_UART", "/dev/ttyUSB0")
  local baud = tonumber(env("GDBFORGE_KGDB_BAUD", "115200")) or 115200

  gdbforge.open_serial_terminal(uart, baud)
  local console = gdbforge.serial_terminal_pty()
  local gdb = gdbforge.serial_debugger_pty()

  -- New mux session: next kgdb_trigger must run target remote once.
  os.remove((os.getenv("HOME") or "") .. "/.cache/gdbforge/kgdb_serial.state")

  gdbforge.print(uart .. " @ " .. baud .. "  (held by gdbforge)")
  gdbforge.print("console PTY: " .. console .. "  ← minicom")
  gdbforge.print("gdb PTY:     " .. gdb .. "  ← target remote HERE")
  gdbforge.print("")
  gdbforge.print("Next: :lua kgdb_trigger   (automated break-in)")
  gdbforge.print("Or manual: :serial-switch gdb → target remote " .. gdb)

  gdbforge.spawn_serial_console(console, baud)
end
