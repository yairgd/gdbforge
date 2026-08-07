-- kgdb over shared UART via external kdmx — one-shot → stopped debug mode.
-- Install: cp -r lua/kgdb_uart lua/kgdb_common .gdbforge/lua/
-- Docs:    docs/KERNEL_KGDB.md
--
-- Usage:   :lua kgdb_uart [module]
-- Example: :lua kgdb_uart 8250
--
-- Env:
--   GDBFORGE_KGDB_UART        serial device (required), e.g. /dev/ttyUSB0
--   GDBFORGE_KGDB_BAUD        default 115200
--   GDBFORGE_KGDB_VMLINUX     path to vmlinux
--   GDBFORGE_KGDB_MODULES     optional dir for lx-symbols
--   GDBFORGE_KGDB_KDMX        kdmx binary (default kdmx)
--   GDBFORGE_KGDB_CONSOLE_CMD optional: program for console PTY (default minicom)
--   GDBFORGE_TERMINAL         emulator for spawn_terminal
--
-- Board must be waiting in kgdb (kgdboc=…,kgdbwait or already broken in).

local function load_common()
  local path = gdbforge.lua_dir() .. "/../kgdb_common/kgdb_common.lua"
  local ok, err = pcall(function()
    dofile(path)
  end)
  if not ok then
    gdbforge.print("ERROR: cannot load kgdb_common from " .. path)
    gdbforge.print(tostring(err))
    gdbforge.print("Install: cp -r lua/kgdb_common lua/kgdb_uart .gdbforge/lua/")
    return false
  end
  return true
end

function help()
  gdbforge.print("kgdb_uart — kdmx + minicom + target remote → stopped debug mode")
  gdbforge.print("Usage: :lua kgdb_uart [module]")
  gdbforge.print("  :lua kgdb_uart 8250")
  gdbforge.print("Setup:")
  gdbforge.print("  export GDBFORGE_KGDB_UART=/dev/ttyUSB0")
  gdbforge.print("  export GDBFORGE_KGDB_VMLINUX=/path/to/vmlinux")
  gdbforge.print("  export GDBFORGE_KGDB_MODULES=/path/to/kernel/build   # optional")
  gdbforge.print("  export GDBFORGE_TERMINAL=mate-terminal")
  gdbforge.print("  # board: kgdboc=<uart>,kgdbwait  (kernel waiting in kgdb)")
  gdbforge.print("  # host:  kdmx on PATH (agent-proxy)")
  gdbforge.print("After attach (stopped): set BP, continue, trigger in minicom")
  gdbforge.print("Docs: docs/KERNEL_KGDB.md")
end

function main(module_name)
  if not load_common() then
    return
  end
  local C = kgdb_common

  local uart = C.env("GDBFORGE_KGDB_UART", "")
  if uart == "" then
    gdbforge.print("ERROR: set GDBFORGE_KGDB_UART=/dev/ttyUSB0")
    return
  end
  local baud = C.env("GDBFORGE_KGDB_BAUD", "115200")
  local vmlinux = C.env("GDBFORGE_KGDB_VMLINUX", "")
  local modules = C.env("GDBFORGE_KGDB_MODULES", "")
  local kdmx = C.env("GDBFORGE_KGDB_KDMX", "kdmx")
  local console_cmd = C.env("GDBFORGE_KGDB_CONSOLE_CMD", "minicom")
  module_name = C.trim(module_name)

  local status = "/tmp/gdbforge-kdmx-" .. tostring(os.time())
  local status_gdb = status .. "_gdb"
  local status_trm = status .. "_trm"

  C.run_cmd("rm -f " .. C.shell_quote(status_gdb) .. " " .. C.shell_quote(status_trm))

  local start = string.format(
    "nohup %s -p %s -b %s -s %s >/tmp/gdbforge-kdmx.log 2>&1 </dev/null & echo $!",
    C.shell_quote(kdmx), C.shell_quote(uart), C.shell_quote(baud), C.shell_quote(status)
  )
  gdbforge.print("starting kdmx on " .. uart .. " @" .. baud .. " …")
  local code, out = C.run_cmd(start)
  if code ~= 0 then
    gdbforge.print("ERROR: failed to start kdmx: " .. C.trim(out))
    return
  end
  local pid = C.trim(out):match("(%d+)")
  if pid then
    gdbforge.print("kdmx pid " .. pid .. "  (log /tmp/gdbforge-kdmx.log)")
  end

  gdbforge.print("waiting for kdmx PTY status files …")
  local gdb_pty = C.wait_file(status_gdb, 15)
  local trm_pty = C.wait_file(status_trm, 15)
  if not gdb_pty or not trm_pty then
    gdbforge.print("ERROR: kdmx did not publish PTYs — check /tmp/gdbforge-kdmx.log")
    gdbforge.print("  expected " .. status_gdb .. " and " .. status_trm)
    return
  end
  gdb_pty = gdb_pty:match("(/dev/pts/%d+)") or gdb_pty
  trm_pty = trm_pty:match("(/dev/pts/%d+)") or trm_pty
  gdbforge.print("console PTY: " .. trm_pty)
  gdbforge.print("gdb PTY:     " .. gdb_pty)

  if console_cmd == "minicom" then
    gdbforge.print("opening minicom on console PTY …")
    gdbforge.spawn_terminal("minicom", "-D", trm_pty)
  else
    gdbforge.print("opening " .. console_cmd .. " on console PTY …")
    gdbforge.spawn_terminal(console_cmd, trm_pty)
  end

  gdbforge.open_buffer("gdb")
  if vmlinux ~= "" then
    gdbforge.print("file " .. vmlinux)
    gdbforge.gdb("file " .. vmlinux)
  else
    gdbforge.print("WARN: GDBFORGE_KGDB_VMLINUX unset — using current GDB symbols")
  end

  gdbforge.print("target remote " .. gdb_pty)
  gdbforge.print("(board must be waiting in kgdb — attach may block until then)")
  gdbforge.gdb("target remote " .. gdb_pty)

  C.load_symbols({
    modules_dir = modules,
    module_name = module_name,
  })

  if module_name ~= "" then
    gdbforge.print("module hint: " .. module_name .. " (set BP, then continue; trigger from minicom)")
  end
  gdbforge.print("kgdb_uart: STOPPED in debug mode — set breakpoints, then continue")
end
